package whatsapp

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"bot-glpi/internal/config"
	"bot-glpi/internal/state"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
)

var startTime = time.Now()

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

	_, err = webDB.Exec(`CREATE TABLE IF NOT EXISTS tickets_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		ticket_id TEXT, 
		title TEXT, 
		requester TEXT, 
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		fmt.Println("🚨 Erro ao criar tabela tickets_history:", err)
	}

	_, err = webDB.Exec(`CREATE TABLE IF NOT EXISTS chat_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		chat_jid TEXT, 
		sender_name TEXT, 
		sender_jid TEXT, 
		message_text TEXT, 
		message_type TEXT, 
		is_from_me INTEGER, 
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		fmt.Println("🚨 Erro ao criar tabela chat_messages:", err)
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

	// ROTA DA TELA DE LOGS (Protegida)
	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		tmpl, err := template.ParseFiles(filepath.Join(webDir, "logs.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar a interface de logs: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	// API PARA OBTER ÚLTIMOS LOGS DO BOT (Protegida)
	http.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		// Opção de download do arquivo completo
		download := r.URL.Query().Get("download")
		if download == "true" {
			data, err := os.ReadFile("db/bot.log")
			if err != nil {
				http.Error(w, "Log não encontrado", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", "attachment; filename=bot.log")
			w.Write(data)
			return
		}

		limitStr := r.URL.Query().Get("limit")
		limit := 150
		if limitStr != "" {
			if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
				limit = parsed
			}
		}

		data, err := os.ReadFile("db/bot.log")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"logs": "Nenhum log disponível ainda ou arquivo de log não encontrado."})
			return
		}

		lines := strings.Split(string(data), "\n")
		if len(lines) > limit {
			lines = lines[len(lines)-limit:]
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"logs": strings.Join(lines, "\n")})
	})

	// API PARA OBTER OS CHAMADOS RECENTES (Protegida)
	http.HandleFunc("/api/tickets/recent", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if webDB == nil {
			http.Error(w, "Banco de dados não disponível", http.StatusInternalServerError)
			return
		}

		rows, err := webDB.Query("SELECT ticket_id, title, requester, created_at FROM tickets_history ORDER BY id DESC LIMIT 5")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao buscar histórico: %v", err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type RecentTicket struct {
			TicketID  string `json:"ticket_id"`
			Title     string `json:"title"`
			Requester string `json:"requester"`
			CreatedAt string `json:"created_at"`
		}

		tickets := []RecentTicket{}
		for rows.Next() {
			var t RecentTicket
			var rawTime string
			if err := rows.Scan(&t.TicketID, &t.Title, &t.Requester, &rawTime); err == nil {
				if parsed, errTime := time.Parse("2006-01-02 15:04:05", rawTime); errTime == nil {
					loc, _ := time.LoadLocation("America/Sao_Paulo")
					if loc != nil {
						parsed = parsed.In(loc)
					}
					t.CreatedAt = parsed.Format("02/01/2006 15:04:05")
				} else {
					t.CreatedAt = rawTime
				}
				tickets = append(tickets, t)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tickets)
	})

	// API PARA RECONECTAR O WHATSAPP (Protegida)
	http.HandleFunc("/api/whatsapp/connect", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		ClientMu.Lock()
		client := GlobalClient
		ClientMu.Unlock()

		if client == nil {
			http.Error(w, "Cliente não inicializado", http.StatusInternalServerError)
			return
		}

		go func() {
			fmt.Println("🔄 Tentando reconectar ao WhatsApp via solicitação web...")
			if client.Store.ID == nil {
				triggerManualQRFlow()
			} else {
				_ = client.Connect()
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Tentando conectar..."})
	})

	// API PARA DESCONECTAR E RESETAR SESSÃO DO WHATSAPP (GERAR NOVO QR CODE) (Protegida)
	http.HandleFunc("/api/whatsapp/logout", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		ClientMu.Lock()
		client := GlobalClient
		ClientMu.Unlock()

		if client == nil {
			http.Error(w, "Cliente não inicializado", http.StatusInternalServerError)
			return
		}

		go func() {
			fmt.Println("⚠️ Solicitado logout/reset de sessão via painel web. Desconectando...")
			// Se o cliente estiver conectado, tenta deslogar para limpar credenciais no servidor
			if client.IsConnected() {
				err := client.Logout(context.Background())
				if err != nil {
					// Fallback: se falhar o logout remoto, desconecta e limpa o ID da store manualmente
					triggerManualQRFlow()
				}
			} else {
				// Se já estiver desconectado, limpa a store local e inicia o QR flow
				triggerManualQRFlow()
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Desconectando e iniciando novo QR Code..."})
	})

	// ROTA DE CHATS DO BOT (Protegido)
	http.HandleFunc("/chats", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		tmpl, err := template.ParseFiles(filepath.Join(webDir, "chats.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar a interface de chats: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	// API PARA LISTAR AS CONVERSAS ATIVAS (Protegida)
	http.HandleFunc("/api/chats", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if webDB == nil {
			http.Error(w, "Banco de dados não disponível", http.StatusInternalServerError)
			return
		}

		query := `
			SELECT c.chat_jid, c.sender_name, c.message_text, c.timestamp
			FROM chat_messages c
			INNER JOIN (
				SELECT chat_jid, MAX(id) as max_id
				FROM chat_messages
				GROUP BY chat_jid
			) m ON c.id = m.max_id
			ORDER BY c.timestamp DESC
		`
		rows, err := webDB.Query(query)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao buscar chats: %v", err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type ChatInfo struct {
			JID        string `json:"jid"`
			Name       string `json:"name"`
			LastMsg    string `json:"last_message"`
			Timestamp  string `json:"timestamp"`
			UserStatus string `json:"status"` // "live_chat", "queue", "bot"
		}

		chats := []ChatInfo{}
		for rows.Next() {
			var c ChatInfo
			var rawTime string
			if err := rows.Scan(&c.JID, &c.Name, &c.LastMsg, &rawTime); err == nil {
				if parsed, errTime := time.Parse("2006-01-02 15:04:05", rawTime); errTime == nil {
					loc, _ := time.LoadLocation("America/Sao_Paulo")
					if loc != nil {
						parsed = parsed.In(loc)
					}
					c.Timestamp = parsed.Format("02/01 15:04")
				} else {
					c.Timestamp = rawTime
				}

				c.UserStatus = "bot"
				userJIDStr := strings.Split(c.JID, "@")[0]
				state.Mu.Lock()
				if uState, exists := state.Users[userJIDStr]; exists {
					if uState.Step == 100 {
						c.UserStatus = "live_chat"
					} else if uState.Step == 99 {
						c.UserStatus = "queue"
					}
				}
				state.Mu.Unlock()

				chats = append(chats, c)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chats)
	})

	// API PARA IMAGEM DE PERFIL DO CONTATO (Protegida)
	http.HandleFunc("/api/chats/avatar", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		jidParam := r.URL.Query().Get("jid")
		nameParam := r.URL.Query().Get("name")
		if nameParam == "" {
			nameParam = "U"
		}

		fallbackURL := fmt.Sprintf("https://ui-avatars.com/api/?name=%s&background=random&color=fff", url.QueryEscape(nameParam))

		if jidParam == "" {
			http.Redirect(w, r, fallbackURL, http.StatusTemporaryRedirect)
			return
		}

		ClientMu.Lock()
		client := GlobalClient
		ClientMu.Unlock()

		if client == nil || !client.IsConnected() {
			http.Redirect(w, r, fallbackURL, http.StatusTemporaryRedirect)
			return
		}

		targetJID, err := types.ParseJID(jidParam)
		if err != nil {
			http.Redirect(w, r, fallbackURL, http.StatusTemporaryRedirect)
			return
		}

		// Obtém a imagem de perfil do WhatsApp
		avatarInfo, err := client.GetProfilePictureInfo(r.Context(), targetJID, nil)
		if err != nil || avatarInfo == nil || avatarInfo.URL == "" {
			http.Redirect(w, r, fallbackURL, http.StatusTemporaryRedirect)
			return
		}

		// Redireciona para o link direto do avatar hospedado nos servidores do WhatsApp
		http.Redirect(w, r, avatarInfo.URL, http.StatusTemporaryRedirect)
	})

	// API PARA OBTER HISTÓRICO DE MENSAGENS (Protegida)
	http.HandleFunc("/api/chats/messages", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if webDB == nil {
			http.Error(w, "Banco de dados não disponível", http.StatusInternalServerError)
			return
		}

		jid := r.URL.Query().Get("jid")
		if jid == "" {
			http.Error(w, "JID é obrigatório", http.StatusBadRequest)
			return
		}

		rows, err := webDB.Query(`
			SELECT id, sender_name, sender_jid, message_text, message_type, is_from_me, timestamp 
			FROM chat_messages 
			WHERE chat_jid = ? 
			ORDER BY id ASC
		`, jid)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao buscar mensagens: %v", err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type MsgInfo struct {
			ID         int    `json:"id"`
			SenderName string `json:"sender_name"`
			SenderJID  string `json:"sender_jid"`
			Text       string `json:"text"`
			Type       string `json:"type"`
			IsFromMe   bool   `json:"is_from_me"`
			Timestamp  string `json:"timestamp"`
		}

		messages := []MsgInfo{}
		for rows.Next() {
			var m MsgInfo
			var rawTime string
			var isFromMeInt int
			if err := rows.Scan(&m.ID, &m.SenderName, &m.SenderJID, &m.Text, &m.Type, &isFromMeInt, &rawTime); err == nil {
				m.IsFromMe = isFromMeInt == 1
				if parsed, errTime := time.Parse("2006-01-02 15:04:05", rawTime); errTime == nil {
					loc, _ := time.LoadLocation("America/Sao_Paulo")
					if loc != nil {
						parsed = parsed.In(loc)
					}
					m.Timestamp = parsed.Format("15:04")
				} else {
					m.Timestamp = rawTime
				}
				messages = append(messages, m)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	})

	// API PARA ENVIAR MENSAGEM DO PAINEL WEB PARA O WHATSAPP (Protegida)
	http.HandleFunc("/api/chats/send", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		ClientMu.Lock()
		client := GlobalClient
		ClientMu.Unlock()

		if client == nil || !client.IsConnected() {
			http.Error(w, "WhatsApp desconectado", http.StatusServiceUnavailable)
			return
		}

		var req struct {
			JID  string `json:"jid"`
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		targetJID, err := types.ParseJID(req.JID)
		if err != nil {
			http.Error(w, "JID inválido", http.StatusBadRequest)
			return
		}

		sendTextMessage(context.Background(), client, targetJID, req.Text)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

	// API PARA TESTAR AS CONFIGURAÇÕES SMTP (Protegido)
	http.HandleFunc("/api/config/test-smtp", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		var testCfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&testCfg); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		if testCfg.SMTPHost == "" || testCfg.SMTPUsername == "" || testCfg.SMTPPassword == "" || testCfg.SMTPSender == "" || testCfg.SMTPReceiver == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": "Preencha todos os campos do SMTP antes de testar."})
			return
		}

		err := SendSMTPTestEmail(testCfg)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

	// API PARA ALTERAR A SENHA DO ADMIN (Protegido)
	http.HandleFunc("/api/change-password", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		if req.NewPassword == "" {
			http.Error(w, "A nova senha não pode ser vazia", http.StatusBadRequest)
			return
		}

		var dbPass string
		err := webDB.QueryRow("SELECT password FROM users WHERE username = 'admin'").Scan(&dbPass)
		if err != nil {
			http.Error(w, "Erro ao buscar senha atual", http.StatusInternalServerError)
			return
		}

		if dbPass != req.CurrentPassword {
			http.Error(w, "Senha atual incorreta", http.StatusBadRequest)
			return
		}

		_, err = webDB.Exec("UPDATE users SET password = ? WHERE username = 'admin'", req.NewPassword)
		if err != nil {
			http.Error(w, "Erro ao atualizar senha", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// API PARA REINICIAR O BOT (Protegido)
	http.HandleFunc("/api/restart", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Bot reiniciando..."})

		go func() {
			fmt.Println("🔄 Solicitação de reinicialização recebida via painel web. Reiniciando o processo...")
			os.WriteFile("db/.restarted", []byte("1"), 0644)
			time.Sleep(1 * time.Second)
			os.Exit(0)
		}()
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

		// Tenta obter o IP de acesso do cabeçalho Host da requisição
		mainIP := r.Host
		if shost, _, err := net.SplitHostPort(r.Host); err == nil {
			mainIP = shost
		}
		// Se for vazio ou loopback, tenta obter o IP local da máquina/container como fallback
		if mainIP == "" || mainIP == "localhost" || mainIP == "127.0.0.1" || mainIP == "::1" {
			ips := getLocalIPs()
			if len(ips) > 0 {
				mainIP = ips[0]
			} else {
				mainIP = "localhost"
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   status,
			"qr":       CurrentQR,
			"ip":       mainIP,
			"engine":   "Whatsmeow (Multi-Device)",
			"timezone": "America/Sao_Paulo (UTC-3)",
			"uptime":   getUptime(),
		})
	})

	// WEBHOOK PARA CHAMADOS ABERTOS POR OUTROS MEIOS (EX: E-MAIL)
	http.HandleFunc("/api/webhook/glpi", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		var rawBody []byte
		if r.Body != nil {
			var err error
			rawBody, err = io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "Erro ao ler corpo da requisição", http.StatusBadRequest)
				return
			}
		}

		fmt.Printf("📥 [WEBHOOK GLPI] Payload recebido: %s\n", string(rawBody))

		var payload struct {
			ID        interface{} `json:"id"`
			TicketID  interface{} `json:"ticket_id"`
			Name      string      `json:"name"`
			Title     string      `json:"title"`
			Requester string      `json:"requester"`
			User      string      `json:"user"`
			Source    string      `json:"source"`
			Origem    string      `json:"origem"`
		}

		if err := json.Unmarshal(rawBody, &payload); err != nil {
			// Se falhar o unmarshal JSON, tenta ler de urlencoded form
			r.ParseForm()
			payload.Title = r.FormValue("title")
			if payload.Title == "" {
				payload.Title = r.FormValue("name")
			}
			payload.Requester = r.FormValue("requester")
			if payload.Requester == "" {
				payload.Requester = r.FormValue("user")
			}
			payload.Source = r.FormValue("source")
			if payload.Source == "" {
				payload.Source = r.FormValue("origem")
			}
			idStr := r.FormValue("ticket_id")
			if idStr == "" {
				idStr = r.FormValue("id")
			}
			if idStr != "" {
				payload.TicketID = idStr
			}
		}

		// Resolve o ID do chamado
		var finalID string
		idVal := payload.TicketID
		if idVal == nil || idVal == "" || idVal == float64(0) {
			idVal = payload.ID
		}
		if idVal != nil {
			switch val := idVal.(type) {
			case float64:
				finalID = fmt.Sprintf("%.0f", val)
			case string:
				finalID = val
			case int:
				finalID = fmt.Sprintf("%d", val)
			}
		}

		if finalID == "" {
			http.Error(w, `{"error": "ID do chamado (id ou ticket_id) não fornecido"}`, http.StatusBadRequest)
			return
		}

		// Resolve Título
		finalTitle := payload.Title
		if finalTitle == "" {
			finalTitle = payload.Name
		}
		if finalTitle == "" {
			finalTitle = "Sem título"
		}

		// Resolve Solicitante
		finalRequester := payload.Requester
		if finalRequester == "" {
			finalRequester = payload.User
		}
		if finalRequester == "" {
			finalRequester = "E-mail Receiver"
		}

		// Resolve Origem (Default: E-mail)
		finalSource := payload.Source
		if finalSource == "" {
			finalSource = payload.Origem
		}
		if finalSource == "" {
			finalSource = "E-mail"
		}

		// Obtém número de notificação da config
		numeroTI := config.GetConfig().TelefoneNotificacao
		if numeroTI == "" {
			http.Error(w, `{"error": "Telefone de notificação não configurado"}`, http.StatusPreconditionFailed)
			return
		}

		ClientMu.Lock()
		client := GlobalClient
		connected := IsConnected
		ClientMu.Unlock()

		if client == nil || !connected {
			http.Error(w, `{"error": "Bot de WhatsApp desconectado"}`, http.StatusServiceUnavailable)
			return
		}

		targetJID := types.NewJID(numeroTI, types.DefaultUserServer)
		linkTicket := obterLinkTicketGLPI(finalID)
		textoAlerta := fmt.Sprintf(
			"🔔 *NOVO CHAMADO VIA %s*\n\n"+
				"📌 *Ticket:* #%s %s\n"+
				"👤 *Solicitante:* %s\n"+
				"📝 *Título:* %s\n\n"+
				"⚠️ _Acesse o painel do GLPI para iniciar as tratativas._",
			strings.ToUpper(finalSource), finalID, linkTicket, finalRequester, finalTitle,
		)

		_, err := client.SendMessage(context.Background(), targetJID, &waE2E.Message{
			Conversation: &textoAlerta,
		})
		if err != nil {
			fmt.Printf("🚨 [WEBHOOK GLPI] Erro ao enviar mensagem para %s: %v\n", numeroTI, err)
			http.Error(w, fmt.Sprintf(`{"error": "Erro ao enviar mensagem: %v"}`, err), http.StatusInternalServerError)
			return
		}

		// Salva no histórico local para exibição no painel
		if webDB != nil {
			_, errDb := webDB.Exec("INSERT INTO tickets_history (ticket_id, title, requester) VALUES (?, ?, ?)", finalID, finalTitle, finalRequester)
			if errDb != nil {
				fmt.Printf("🚨 [DATABASE] Erro ao salvar ticket no histórico: %v\n", errDb)
			}
		}

		fmt.Printf("✅ [WEBHOOK GLPI] Alerta do chamado #%s enviado com sucesso para %s\n", finalID, numeroTI)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	fmt.Printf("🌐 Painel Web protegido rodando em:\n")
	fmt.Printf("   - http://localhost:33090\n")
	for _, ip := range getLocalIPs() {
		fmt.Printf("   - http://%s:33090\n", ip)
	}

	// Usa o recoveryHandler para interceptar panics e mostrar erro no navegador
	if err := http.ListenAndServe(porta, &recoveryHandler{handler: http.DefaultServeMux}); err != nil {
		fmt.Printf("🚨 Erro fatal no servidor web: %v\n", err)
	}
}

// Retorna todos os IPs de rede locais (não-loopback) do host
func getLocalIPs() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return []string{"127.0.0.1"}
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips
}

// Formata e retorna o tempo de atividade do bot (uptime)
func getUptime() string {
	d := time.Since(startTime)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func triggerManualQRFlow() {
	IsConnected = false
	CurrentQR = ""
	
	ClientMu.Lock()
	client := GlobalClient
	ClientMu.Unlock()

	if client != nil {
		client.Disconnect()
		_ = client.Store.Delete(context.Background())
	}

	go func() {
		fmt.Println("🔄 Inicializando novo canal de QR Code para re-pareamento manual...")
		err := StartWhatsApp(context.Background())
		if err != nil {
			fmt.Printf("🚨 Erro ao reiniciar cliente WhatsApp no pareamento manual: %v\n", err)
		}
	}()
}