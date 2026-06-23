package whatsapp

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"bot-glpi/internal/config"
)

// recoveryHandler intercepta panics e exibe o erro no navegador em vez de resposta vazia
type recoveryHandler struct {
	handler http.Handler
}

func (h *recoveryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("\n🚨 PANIC em %s %s:\n%v\n%s\n", r.Method, r.URL.Path, err, debug.Stack())
			http.Error(w, fmt.Sprintf("Erro interno do servidor: %v", err), http.StatusInternalServerError)
		}
	}()
	h.handler.ServeHTTP(w, r)
}

// Inicializa o banco de dados do painel web e cria o usuário padrão
func initWebDB() {
	os.MkdirAll("db", 0777)
	var err error
	webDB, err = sql.Open("sqlite", "db/web.db")
	if err != nil {
		fmt.Println("🚨 Erro ao iniciar DB do Painel Web:", err)
		return
	}

	// Testa a conexão real com o banco
	if err = webDB.Ping(); err != nil {
		fmt.Println("🚨 Erro ao conectar no DB do Painel Web:", err)
		webDB = nil
		return
	}

	_, err = webDB.Exec(`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, username TEXT UNIQUE, password TEXT)`)
	if err != nil {
		fmt.Println("🚨 Erro ao criar tabela users:", err)
	}

	var user string
	err = webDB.QueryRow("SELECT username FROM users WHERE username = 'admin'").Scan(&user)
	if err != nil {
		// Se não achar o admin, insere a conta padrão
		_, err = webDB.Exec("INSERT INTO users (username, password) VALUES ('admin', 'admin123')")
		if err != nil {
			fmt.Println("🚨 Erro ao criar usuário admin:", err)
		} else {
			fmt.Println("✅ Usuário administrador padrão criado com sucesso no banco de dados.")
		}
	}

	fmt.Println("✅ Banco de dados do Painel Web inicializado com sucesso.")
}

// Gera um token de sessão aleatório
func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Verifica se o usuário tem um cookie válido
func isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return false
	}
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	expiry, exists := sessions[cookie.Value]
	if !exists || time.Now().After(expiry) {
		return false
	}
	return true
}

func getWebDir() string {
	paths := []string{".", "..", "../..", "../../.."}
	for _, p := range paths {
		webPath := filepath.Join(p, "web")
		if stat, err := os.Stat(webPath); err == nil && stat.IsDir() {
			return p
		}
	}
	return "."
}

func StartWebServer() {
	initWebDB()
	porta := "0.0.0.0:33090"
	basePath := getWebDir()
	webDir := filepath.Join(basePath, "web")
	staticDir := filepath.Join(basePath, "static")

	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// ROTA DE LOGIN
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			tmpl, err := template.ParseFiles(filepath.Join(webDir, "login.html"))
			if err != nil {
				http.Error(w, fmt.Sprintf("Erro ao carregar login: %v", err), http.StatusInternalServerError)
				return
			}
			tmpl.Execute(w, map[string]string{})
			return
		}

		if r.Method == "POST" {
			// Verificação de segurança: se o banco não foi inicializado
			if webDB == nil {
				fmt.Println("🚨 ERRO CRÍTICO: webDB é nil no momento do login!")
				http.Error(w, "Erro interno: banco de dados do painel não foi inicializado. Verifique os logs do container.", http.StatusInternalServerError)
				return
			}

			r.ParseForm()
			username := r.FormValue("username")
			password := r.FormValue("password")

			var dbPass string
			err := webDB.QueryRow("SELECT password FROM users WHERE username = ?", username).Scan(&dbPass)

			if err == nil && dbPass == password {
				// Login com Sucesso
				token := generateToken()
				sessionsMu.Lock()
				sessions[token] = time.Now().Add(24 * time.Hour)
				sessionsMu.Unlock()

				http.SetCookie(w, &http.Cookie{
					Name:    "session_token",
					Value:   token,
					Path:    "/",
					Expires: time.Now().Add(24 * time.Hour),
				})
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}

			// Login Falhou
			tmpl, err := template.ParseFiles(filepath.Join(webDir, "login.html"))
			if err != nil {
				http.Error(w, fmt.Sprintf("Erro ao carregar login: %v", err), http.StatusInternalServerError)
				return
			}
			tmpl.Execute(w, map[string]string{"Error": "Usuário ou senha inválidos."})
		}
	})

	// Rota para fazer logout
	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:    "session_token",
			Value:   "",
			Path:    "/",
			Expires: time.Now().Add(-1 * time.Hour), // Apaga o cookie
		})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})

	// ROTA PRINCIPAL (Dashboard Protegido)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		// Trava de Segurança
		if !isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		tmpl, err := template.ParseFiles(filepath.Join(webDir, "index.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar a interface: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	// ROTA DE CONFIGURAÇÕES GERAIS (Protegido)
	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		tmpl, err := template.ParseFiles(filepath.Join(webDir, "config.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar a interface: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	// ROTA DE MENSAGENS DO SISTEMA (Protegido)
	http.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		tmpl, err := template.ParseFiles(filepath.Join(webDir, "messages.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar a interface: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	// API PARA OBTER E SALVAR CONFIGURAÇÕES GERAIS
	http.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			cfg := config.GetConfig()
			json.NewEncoder(w).Encode(cfg)
			return
		}

		if r.Method == "POST" {
			var newCfg config.Config
			if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
				http.Error(w, "JSON inválido", http.StatusBadRequest)
				return
			}

			if err := config.SaveConfig(newCfg); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}

		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
	})

	// ROTA DE CONSTRUTOR DE FLUXO (Protegido)
	http.HandleFunc("/flow", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		tmpl, err := template.ParseFiles(filepath.Join(webDir, "flow.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar a interface: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	// API PARA OBTER E SALVAR FLUXO DE CONVERSA
	http.HandleFunc("/api/flow", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			flow := GetFlowConfig()
			json.NewEncoder(w).Encode(flow)
			return
		}

		if r.Method == "POST" {
			var newFlow FlowNode
			if err := json.NewDecoder(r.Body).Decode(&newFlow); err != nil {
				http.Error(w, "JSON inválido", http.StatusBadRequest)
				return
			}

			if err := SaveFlowConfig(newFlow); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}

		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
	})

	// ROTA DE STATUS DA API
	http.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := "waiting"
		if IsConnected {
			status = "connected"
		} else if CurrentQR != "" {
			status = "qr"
		}

		json.NewEncoder(w).Encode(map[string]string{
			"status": status,
			"qr":     CurrentQR,
		})
	})

	fmt.Printf("🌐 Painel Web protegido rodando em http://%s\n", porta)
	// Usa o recoveryHandler para interceptar panics e mostrar erro no navegador
	if err := http.ListenAndServe(porta, &recoveryHandler{handler: http.DefaultServeMux}); err != nil {
		fmt.Printf("🚨 Erro fatal no servidor web: %v\n", err)
	}
}