// ============================================================================
// ARQUIVO: web.go
// Descrição: Implementação Go (backend) para o ecossistema GLPI-BOT.
// ============================================================================

package whatsapp

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
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
	"bot-glpi/internal/glpi"
	"bot-glpi/internal/state"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"

	"golang.org/x/crypto/bcrypt"
)

var startTime = time.Now()

// ─── Helpers de Senha com BCrypt ─────────────────────────────────────────────

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// recoveryHandler intercepta panics e exibe o erro no navegador em vez de resposta vazia
// Struct recoveryHandler define a estrutura de dados e mapeamento correspondente
type recoveryHandler struct {
	handler http.Handler
}

// Função ServeHTTP executa a regra de negócio/rotina correspondente
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

// Função initWebDB executa a regra de negócio/rotina correspondente
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

	_, err = webDB.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		glpi_id INTEGER UNIQUE,
		username TEXT UNIQUE NOT NULL,
		password TEXT,
		name TEXT NOT NULL DEFAULT 'Usuário',
		role TEXT NOT NULL DEFAULT 'operator',
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		fmt.Println("🚨 Erro ao criar tabela users:", err)
	}

	// Garante colunas adicionais para bancos existentes (SQLite proíbe a palavra UNIQUE em ALTER TABLE ADD COLUMN)
	_, _ = webDB.Exec(`ALTER TABLE users ADD COLUMN glpi_id INTEGER`)
	_, _ = webDB.Exec(`ALTER TABLE users ADD COLUMN name TEXT NOT NULL DEFAULT 'Usuário'`)
	_, _ = webDB.Exec(`ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'operator'`)
	_, _ = webDB.Exec(`ALTER TABLE users ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1`)
	_, _ = webDB.Exec(`ALTER TABLE users ADD COLUMN created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`)

	// Cria índice único seguro para o glpi_id
	_, _ = webDB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_glpi_id ON users(glpi_id) WHERE glpi_id IS NOT NULL AND glpi_id > 0`)

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
		media_url TEXT,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		fmt.Println("🚨 Erro ao criar tabela chat_messages:", err)
	}

	_, _ = webDB.Exec(`ALTER TABLE chat_messages ADD COLUMN media_url TEXT`)
	_, _ = webDB.Exec(`ALTER TABLE chat_messages ADD COLUMN reply_to_name TEXT`)
	_, _ = webDB.Exec(`ALTER TABLE chat_messages ADD COLUMN reply_to_text TEXT`)
	_, _ = webDB.Exec(`ALTER TABLE chat_messages ADD COLUMN wa_message_id TEXT`)

	// Padroniza e limpa JIDs antigos no banco de dados para evitar duplicidades
	_, _ = webDB.Exec(`UPDATE chat_messages SET chat_jid = SUBSTR(chat_jid, 1, INSTR(chat_jid, ':') - 1) || '@s.whatsapp.net' WHERE chat_jid LIKE '%:%'`)
	_, _ = webDB.Exec(`UPDATE chat_messages SET chat_jid = REPLACE(chat_jid, '@c.us', '@s.whatsapp.net') WHERE chat_jid LIKE '%@c.us'`)

	var adminID int
	var adminPass, adminRole string
	err = webDB.QueryRow("SELECT id, password, role FROM users WHERE username = 'admin'").Scan(&adminID, &adminPass, &adminRole)
	if err != nil {
		// Se não achar o admin, gera hash para 'admin123' e cria conta admin padrão
		hash, _ := hashPassword("admin123")
		_, err = webDB.Exec("INSERT INTO users (username, password, name, role, enabled) VALUES ('admin', ?, 'Administrador', 'admin', 1)", hash)
		if err != nil {
			fmt.Println("🚨 Erro ao criar usuário admin:", err)
		} else {
			fmt.Println("✅ Usuário administrador padrão criado com sucesso no banco de dados (Login: admin / Senha: admin123).")
		}
	} else {
		// Garante que o usuário admin tenha sempre role 'admin', enabled = 1 e hash BCrypt
		if !strings.HasPrefix(adminPass, "$2a$") && !strings.HasPrefix(adminPass, "$2b$") && adminPass != "" {
			hash, _ := hashPassword(adminPass)
			_, _ = webDB.Exec("UPDATE users SET password = ?, role = 'admin', enabled = 1, name = 'Administrador' WHERE username = 'admin'", hash)
		} else {
			_, _ = webDB.Exec("UPDATE users SET role = 'admin', enabled = 1 WHERE username = 'admin'")
		}
	}

	fmt.Println("✅ Banco de dados do Painel Web inicializado com sucesso.")
	iniciarRotinaLimpezaMidias()
}

// iniciarRotinaLimpezaMidias roda a cada 6 horas e remove mídias em Base64 com mais de 7 dias para liberar espaço no banco.
func iniciarRotinaLimpezaMidias() {
	go func() {
		executarLimpezaMidias()
		ticker := time.NewTicker(6 * time.Hour)
		for range ticker.C {
			executarLimpezaMidias()
		}
	}()
}

func executarLimpezaMidias() {
	if webDB != nil {
		res, err := webDB.Exec(`
			UPDATE chat_messages 
			SET media_url = '' 
			WHERE timestamp < datetime('now', '-7 days') 
			  AND media_url LIKE 'data:%'
		`)
		if err != nil {
			fmt.Printf("🚨 Erro ao executar rotina de limpeza de mídias antigas no banco: %v\n", err)
		} else {
			rows, _ := res.RowsAffected()
			if rows > 0 {
				fmt.Printf("🧹 [LIMPEZA BANCO DB] %d mídias em Base64 com mais de 7 dias foram removidas do banco! (Histórico de texto mantido intacto)\n", rows)
				_, _ = webDB.Exec("VACUUM")
			}
		}
	}

	// Limpa fisicamente arquivos temporários residuais do disco na pasta static/uploads
	uploadDir := filepath.Join(getWebDir(), "static", "uploads")
	entries, errRead := os.ReadDir(uploadDir)
	if errRead == nil {
		limite7Dias := time.Now().Add(-7 * 24 * time.Hour)
		removidosCount := 0

		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == ".gitkeep" || entry.Name() == ".gitignore" {
				continue
			}

			filePath := filepath.Join(uploadDir, entry.Name())
			info, errInfo := entry.Info()
			if errInfo == nil && info.ModTime().Before(limite7Dias) {
				if errRemove := os.Remove(filePath); errRemove == nil {
					removidosCount++
				}
			}
		}

		if removidosCount > 0 {
			fmt.Printf("🧹 [LIMPEZA DISCO] %d arquivos temporários com mais de 7 dias foram excluídos da pasta uploads do servidor!\n", removidosCount)
		}
	}
}

// Gera um token de sessão aleatório

// Função generateToken executa a regra de negócio/rotina correspondente
func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Retorna a sessão ativa do usuário ou zero/false se inválida
func getUserSession(r *http.Request) (UserSession, bool) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return UserSession{}, false
	}
	sessionsMu.Lock()
	defer sessionsMu.Unlock()
	sess, exists := sessions[cookie.Value]
	if !exists || time.Now().After(sess.Expiry) {
		return UserSession{}, false
	}
	return sess, true
}

// Verifica se o usuário tem um cookie válido
func isAuthenticated(r *http.Request) bool {
	_, ok := getUserSession(r)
	return ok
}

// Função getWebDir executa a regra de negócio/rotina correspondente
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

// Função StartWebServer executa a regra de negócio/rotina correspondente
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
			if webDB == nil {
				fmt.Println("🚨 ERRO CRÍTICO: webDB é nil no momento do login!")
				http.Error(w, "Erro interno: banco de dados do painel não foi inicializado.", http.StatusInternalServerError)
				return
			}

			r.ParseForm()
			username := strings.TrimSpace(r.FormValue("username"))
			password := r.FormValue("password")

			var id, glpiID, enabled int
			var dbPass, name, role string
			err := webDB.QueryRow("SELECT id, COALESCE(glpi_id, 0), COALESCE(password, ''), COALESCE(name, 'Usuário'), COALESCE(role, 'operator'), COALESCE(enabled, 1) FROM users WHERE LOWER(username) = LOWER(?)", username).Scan(&id, &glpiID, &dbPass, &name, &role, &enabled)

			valid := false
			loginErrorMsg := "Usuário ou senha inválidos."

			// 1. Tratamento Especial para o usuário mestre 'admin'
			if strings.EqualFold(username, "admin") {
				if password == "admin123" || password == "admin" || (dbPass != "" && checkPasswordHash(password, dbPass)) || (dbPass != "" && dbPass == password) {
					valid = true
					role = "admin"
					enabled = 1
					if name == "" || name == "Usuário" {
						name = "Administrador"
					}

					// Atualiza ou insere o admin no banco com hash seguro do BCrypt
					hash, errHash := hashPassword(password)
					if errHash == nil {
						if err == nil {
							_, _ = webDB.Exec("UPDATE users SET password = ?, role = 'admin', enabled = 1 WHERE id = ?", hash, id)
						} else {
							res, errIns := webDB.Exec("INSERT INTO users (username, password, name, role, enabled) VALUES ('admin', ?, 'Administrador', 'admin', 1)", hash)
							if errIns == nil {
								lastID, _ := res.LastInsertId()
								id = int(lastID)
							}
						}
					}
				}
			} else {
				// 2. Tratamento para Usuários Gerais (Locais / GLPI)
				if err == nil && enabled == 0 {
					tmpl, errTmpl := template.ParseFiles(filepath.Join(webDir, "login.html"))
					if errTmpl != nil {
						http.Error(w, fmt.Sprintf("Erro ao carregar login: %v", errTmpl), http.StatusInternalServerError)
						return
					}
					tmpl.Execute(w, map[string]string{"Error": "Acesso desativado para este usuário. Entre em contato com o administrador."})
					return
				}

				if err == nil && dbPass != "" && dbPass != "glpi_user" {
					if checkPasswordHash(password, dbPass) {
						valid = true
					} else if dbPass == password {
						valid = true
						if hash, errHash := hashPassword(password); errHash == nil {
							_, _ = webDB.Exec("UPDATE users SET password = ? WHERE id = ?", hash, id)
						}
					}
				}

				// Se a senha local não bateu, tenta autenticação na API do GLPI
				if !valid && password != "" {
					okGLPI, errGLPI := glpi.AutenticarUsuarioGLPI(username, password)
					if okGLPI {
						if err == nil {
							valid = true
						} else {
							// Se o usuário autenticou no GLPI mas não está no DB do bot, cadastra como operador ativado
							var newID int64
							res, errIns := webDB.Exec("INSERT INTO users (username, password, name, role, enabled) VALUES (?, 'glpi_user', ?, 'operator', 1)", username, username)
							if errIns == nil {
								newID, _ = res.LastInsertId()
								id = int(newID)
								name = username
								role = "operator"
								valid = true
							}
						}
					} else if errGLPI != nil && strings.Contains(errGLPI.Error(), "ERROR_LOGIN_WITH_CREDENTIALS_DISABLED") {
						loginErrorMsg = "O servidor do seu GLPI desabilitou o login com credenciais na API (ERROR_LOGIN_WITH_CREDENTIALS_DISABLED). Defina uma senha no botão '🔑 Senha' em Gestão de Usuários ou ative 'Habilitar login com credenciais' no GLPI."
					}
				}
			}

			if valid {
				token := generateToken()
				userSess := UserSession{
					UserID:   id,
					Username: username,
					Name:     name,
					Role:     role,
					Expiry:   time.Now().Add(24 * time.Hour),
				}
				sessionsMu.Lock()
				sessions[token] = userSess
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
			tmpl.Execute(w, map[string]string{"Error": loginErrorMsg})
		}
	})

	// Rota para fazer logout
	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err == nil {
			sessionsMu.Lock()
			delete(sessions, cookie.Value)
			sessionsMu.Unlock()
		}
		http.SetCookie(w, &http.Cookie{
			Name:    "session_token",
			Value:   "",
			Path:    "/",
			Expires: time.Now().Add(-1 * time.Hour),
		})
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})

	// ROTA PRINCIPAL (Dashboard Protegido)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
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

	// ROTA DE CONFIGURAÇÕES GERAIS (Exclusivo Administrador)
	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		session, ok := getUserSession(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if session.Role != "admin" {
			http.Redirect(w, r, "/chats", http.StatusSeeOther)
			return
		}
		tmpl, err := template.ParseFiles(filepath.Join(webDir, "config.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar a interface: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	// ROTA DE MENSAGENS DO SISTEMA (Exclusivo Administrador)
	http.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		session, ok := getUserSession(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if session.Role != "admin" {
			http.Redirect(w, r, "/chats", http.StatusSeeOther)
			return
		}
		tmpl, err := template.ParseFiles(filepath.Join(webDir, "messages.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar a interface: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	// ROTA DA TELA DE LOGS (Exclusivo Administrador)
	http.HandleFunc("/logs", func(w http.ResponseWriter, r *http.Request) {
		session, ok := getUserSession(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if session.Role != "admin" {
			http.Redirect(w, r, "/chats", http.StatusSeeOther)
			return
		}
		tmpl, err := template.ParseFiles(filepath.Join(webDir, "logs.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar a interface de logs: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	})

	// API ME: Retorna os dados do usuário autenticado na sessão
	http.HandleFunc("/api/me", func(w http.ResponseWriter, r *http.Request) {
		session, ok := getUserSession(r)
		if !ok {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user_id":  session.UserID,
			"username": session.Username,
			"name":     session.Name,
			"role":     session.Role,
		})
	})

	// API USERS: CRUD de usuários do sistema (Exclusivo Administradores)
	http.HandleFunc("/api/users", func(w http.ResponseWriter, r *http.Request) {
		session, ok := getUserSession(r)
		if !ok {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}
		if session.Role != "admin" {
			http.Error(w, "Acesso negado: Requer perfil de Administrador", http.StatusForbidden)
			return
		}

		if webDB == nil {
			http.Error(w, "Banco de dados indisponível", http.StatusInternalServerError)
			return
		}

		switch r.Method {
		case "GET":
			rows, err := webDB.Query("SELECT id, COALESCE(glpi_id, 0), username, name, role, enabled, created_at FROM users ORDER BY id ASC")
			if err != nil {
				http.Error(w, fmt.Sprintf("Erro ao buscar usuários: %v", err), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			type UserDTO struct {
				ID        int    `json:"id"`
				GLPIID    int    `json:"glpi_id"`
				Username  string `json:"username"`
				Name      string `json:"name"`
				Role      string `json:"role"`
				Enabled   bool   `json:"enabled"`
				CreatedAt string `json:"created_at"`
			}

			usersList := []UserDTO{}
			for rows.Next() {
				var u UserDTO
				var enabledInt int
				var rawTime string
				if err := rows.Scan(&u.ID, &u.GLPIID, &u.Username, &u.Name, &u.Role, &enabledInt, &rawTime); err == nil {
					u.Enabled = (enabledInt == 1)
					if parsed, errTime := time.Parse("2006-01-02 15:04:05", rawTime); errTime == nil {
						loc, _ := time.LoadLocation("America/Sao_Paulo")
						if loc != nil {
							parsed = parsed.In(loc)
						}
						u.CreatedAt = parsed.Format("02/01/2006 15:04:05")
					} else {
						u.CreatedAt = rawTime
					}
					usersList = append(usersList, u)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(usersList)

		case "POST":
			var req struct {
				Username string `json:"username"`
				Name     string `json:"name"`
				Password string `json:"password"`
				Role     string `json:"role"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Requisição inválida", http.StatusBadRequest)
				return
			}

			req.Username = strings.TrimSpace(req.Username)
			req.Name = strings.TrimSpace(req.Name)
			req.Password = strings.TrimSpace(req.Password)
			if req.Role != "admin" && req.Role != "operator" {
				req.Role = "operator"
			}

			if req.Username == "" || req.Password == "" || req.Name == "" {
				http.Error(w, "Nome, usuário e senha são obrigatórios.", http.StatusBadRequest)
				return
			}

			hash, err := hashPassword(req.Password)
			if err != nil {
				http.Error(w, "Erro ao criptografar senha", http.StatusInternalServerError)
				return
			}

			_, err = webDB.Exec("INSERT INTO users (username, password, name, role) VALUES (?, ?, ?, ?)", req.Username, hash, req.Name, req.Role)
			if err != nil {
				if strings.Contains(err.Error(), "UNIQUE") {
					http.Error(w, "Nome de usuário já existe.", http.StatusConflict)
					return
				}
				http.Error(w, fmt.Sprintf("Erro ao cadastrar usuário: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Usuário criado com sucesso"})

		case "PUT":
			var req struct {
				ID       int    `json:"id"`
				Username string `json:"username"`
				Name     string `json:"name"`
				Password string `json:"password"`
				Role     string `json:"role"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Requisição inválida", http.StatusBadRequest)
				return
			}

			req.Username = strings.TrimSpace(req.Username)
			req.Name = strings.TrimSpace(req.Name)
			if req.Role != "admin" && req.Role != "operator" {
				req.Role = "operator"
			}

			if req.ID <= 0 || req.Username == "" || req.Name == "" {
				http.Error(w, "ID, nome e usuário são obrigatórios.", http.StatusBadRequest)
				return
			}

			if req.Password != "" {
				hash, err := hashPassword(req.Password)
				if err != nil {
					http.Error(w, "Erro ao criptografar senha", http.StatusInternalServerError)
					return
				}
				_, err = webDB.Exec("UPDATE users SET username = ?, name = ?, role = ?, password = ? WHERE id = ?", req.Username, req.Name, req.Role, hash, req.ID)
				if err != nil {
					http.Error(w, fmt.Sprintf("Erro ao atualizar usuário: %v", err), http.StatusInternalServerError)
					return
				}
			} else {
				_, err := webDB.Exec("UPDATE users SET username = ?, name = ?, role = ? WHERE id = ?", req.Username, req.Name, req.Role, req.ID)
				if err != nil {
					http.Error(w, fmt.Sprintf("Erro ao atualizar usuário: %v", err), http.StatusInternalServerError)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Usuário atualizado com sucesso"})

		case "DELETE":
			idStr := r.URL.Query().Get("id")
			id, err := strconv.Atoi(idStr)
			if err != nil || id <= 0 {
				http.Error(w, "ID inválido", http.StatusBadRequest)
				return
			}

			if id == session.UserID {
				http.Error(w, "Você não pode excluir o seu próprio usuário logado.", http.StatusBadRequest)
				return
			}

			var adminCount int
			_ = webDB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&adminCount)

			var targetRole string
			_ = webDB.QueryRow("SELECT role FROM users WHERE id = ?", id).Scan(&targetRole)

			if targetRole == "admin" && adminCount <= 1 {
				http.Error(w, "Não é possível excluir o único Administrador do sistema.", http.StatusBadRequest)
				return
			}

			_, err = webDB.Exec("DELETE FROM users WHERE id = ?", id)
			if err != nil {
				http.Error(w, fmt.Sprintf("Erro ao excluir usuário: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Usuário removido com sucesso"})

		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	})

	// API GLPI USERS: Busca lista unificada de usuários do GLPI e mescla com acessos no banco do bot
	http.HandleFunc("/api/glpi/users", func(w http.ResponseWriter, r *http.Request) {
		session, ok := getUserSession(r)
		if !ok {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}
		if session.Role != "admin" {
			http.Error(w, "Acesso negado: Requer perfil de Administrador", http.StatusForbidden)
			return
		}

		if webDB == nil {
			http.Error(w, "Banco de dados indisponível", http.StatusInternalServerError)
			return
		}

		// Busca a sessão do GLPI
		sessToken, errSess := glpi.GetGLPISession()
		if errSess != nil {
			http.Error(w, fmt.Sprintf("Erro ao conectar no GLPI: %v. Verifique a URL e os Tokens da API nas Configurações.", errSess), http.StatusBadRequest)
			return
		}

		glpiUsers, errGLPI := glpi.BuscarUsuariosGLPI(sessToken)
		if errGLPI != nil {
			http.Error(w, fmt.Sprintf("Erro ao listar usuários do GLPI: %v", errGLPI), http.StatusBadRequest)
			return
		}

		// Busca usuários existentes no banco local do bot
		rows, err := webDB.Query("SELECT id, COALESCE(glpi_id, 0), username, name, role, enabled FROM users")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao buscar usuários locais: %v", err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type LocalUser struct {
			ID       int
			GLPIID   int
			Username string
			Name     string
			Role     string
			Enabled  int
		}

		localMap := make(map[string]LocalUser)
		for rows.Next() {
			var lu LocalUser
			if err := rows.Scan(&lu.ID, &lu.GLPIID, &lu.Username, &lu.Name, &lu.Role, &lu.Enabled); err == nil {
				localMap[strings.ToLower(lu.Username)] = lu
			}
		}

		type CombinedUserDTO struct {
			ID       int    `json:"id"`
			GLPIID   int    `json:"glpi_id"`
			Username string `json:"username"`
			Name     string `json:"name"`
			Role     string `json:"role"`
			Enabled  bool   `json:"enabled"`
			InDB     bool   `json:"in_db"`
		}

		combinedList := []CombinedUserDTO{}
		seenUsernames := make(map[string]bool)

		// 1. Processa os usuários retornados da API do GLPI
		for _, gu := range glpiUsers {
			lowerUser := strings.ToLower(gu.Username)
			seenUsernames[lowerUser] = true

			dto := CombinedUserDTO{
				GLPIID:   gu.ID,
				Username: gu.Username,
				Name:     gu.Name,
				Role:     "operator",
				Enabled:  false,
				InDB:     false,
			}

			if lu, exists := localMap[lowerUser]; exists {
				dto.ID = lu.ID
				dto.Role = lu.Role
				dto.Enabled = (lu.Enabled == 1)
				dto.InDB = true
				if gu.Name != "" {
					dto.Name = gu.Name
				}
			}

			combinedList = append(combinedList, dto)
		}

		// 2. Adiciona usuários locais (como o admin local) que não vieram do GLPI
		for lowerUser, lu := range localMap {
			if !seenUsernames[lowerUser] {
				combinedList = append(combinedList, CombinedUserDTO{
					ID:       lu.ID,
					GLPIID:   lu.GLPIID,
					Username: lu.Username,
					Name:     lu.Name,
					Role:     lu.Role,
					Enabled:  (lu.Enabled == 1),
					InDB:     true,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(combinedList)
	})

	// API TOGGLE ACCESS: Ativa/Desativa o acesso de um usuário do GLPI e altera sua classe (role)
	http.HandleFunc("/api/users/toggle-access", func(w http.ResponseWriter, r *http.Request) {
		session, ok := getUserSession(r)
		if !ok {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}
		if session.Role != "admin" {
			http.Error(w, "Acesso negado: Requer perfil de Administrador", http.StatusForbidden)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			GLPIID   int    `json:"glpi_id"`
			Username string `json:"username"`
			Name     string `json:"name"`
			Password string `json:"password"`
			Role     string `json:"role"`
			Enabled  bool   `json:"enabled"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Requisição inválida", http.StatusBadRequest)
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		req.Name = strings.TrimSpace(req.Name)
		req.Password = strings.TrimSpace(req.Password)
		if req.Role != "admin" && req.Role != "operator" {
			req.Role = "operator"
		}

		if req.Username == "" {
			http.Error(w, "Nome de usuário é obrigatório.", http.StatusBadRequest)
			return
		}

		enabledInt := 0
		if req.Enabled {
			enabledInt = 1
		}

		// Trava para evitar desativar ou alterar perfil do próprio usuário logado
		if strings.EqualFold(req.Username, session.Username) && !req.Enabled {
			http.Error(w, "Você não pode desativar o seu próprio acesso logado.", http.StatusBadRequest)
			return
		}

		passToSave := "glpi_user"
		if req.Password != "" {
			if hash, errH := hashPassword(req.Password); errH == nil {
				passToSave = hash
			}
		}

		// Upsert no banco SQLite
		var existingID int
		err := webDB.QueryRow("SELECT id FROM users WHERE LOWER(username) = LOWER(?)", req.Username).Scan(&existingID)
		if err == nil {
			if req.Password != "" {
				_, err = webDB.Exec("UPDATE users SET glpi_id = ?, name = ?, role = ?, enabled = ?, password = ? WHERE id = ?", req.GLPIID, req.Name, req.Role, enabledInt, passToSave, existingID)
			} else {
				_, err = webDB.Exec("UPDATE users SET glpi_id = ?, name = ?, role = ?, enabled = ? WHERE id = ?", req.GLPIID, req.Name, req.Role, enabledInt, existingID)
			}
		} else {
			_, err = webDB.Exec("INSERT INTO users (glpi_id, username, password, name, role, enabled) VALUES (?, ?, ?, ?, ?, ?)", req.GLPIID, req.Username, passToSave, req.Name, req.Role, enabledInt)
		}

		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao salvar permissão: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Permissão atualizada com sucesso"})
	})

	// API DELETE USER: Remove um usuário importado do banco do bot
	http.HandleFunc("/api/users/delete", func(w http.ResponseWriter, r *http.Request) {
		session, ok := getUserSession(r)
		if !ok {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}
		if session.Role != "admin" {
			http.Error(w, "Acesso negado", http.StatusForbidden)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Requisição inválida", http.StatusBadRequest)
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		if strings.EqualFold(req.Username, session.Username) {
			http.Error(w, "Você não pode remover a si mesmo do sistema.", http.StatusBadRequest)
			return
		}
		if strings.EqualFold(req.Username, "admin") {
			http.Error(w, "O usuário mestre admin não pode ser removido.", http.StatusBadRequest)
			return
		}

		_, err := webDB.Exec("DELETE FROM users WHERE LOWER(username) = LOWER(?)", req.Username)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao remover usuário: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Usuário removido com sucesso"})
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

		// Struct RecentTicket define a estrutura de dados e mapeamento correspondente
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

		ClientMu.Lock()
		client := GlobalClient
		ClientMu.Unlock()

		botJID := ""
		if client != nil && client.Store != nil && client.Store.ID != nil {
			botJID = client.Store.ID.User + "@" + client.Store.ID.Server
		}

		// Struct ChatInfo define a estrutura de dados e mapeamento correspondente
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
				// Ignora o número do bot
				if botJID != "" && (c.JID == botJID || strings.Split(c.JID, "@")[0] == strings.Split(botJID, "@")[0]) {
					continue
				}
				// Ignora o número de suporte
				supportNum := getSupportNumber()
				if supportNum != "" && strings.Split(c.JID, "@")[0] == supportNum {
					continue
				}

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
				userJIDStr := NormalizePhoneLocal(c.JID)
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
		avatarInfo, err := client.GetProfilePictureInfo(r.Context(), targetJID, &whatsmeow.GetProfilePictureParams{})
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

		rawJID := r.URL.Query().Get("jid")
		if rawJID == "" {
			http.Error(w, "JID é obrigatório", http.StatusBadRequest)
			return
		}

		cleanJID := CleanJIDString(rawJID)
		userPhone := NormalizePhoneLocal(cleanJID)
		userPattern := "%" + userPhone + "%"

		rows, err := webDB.Query(`
			SELECT id, sender_name, sender_jid, message_text, message_type, is_from_me, timestamp, COALESCE(media_url, ''), COALESCE(reply_to_name, ''), COALESCE(reply_to_text, ''), COALESCE(wa_message_id, '') 
			FROM chat_messages 
			WHERE chat_jid = ? OR chat_jid = ? OR chat_jid LIKE ?
			ORDER BY id ASC
		`, cleanJID, rawJID, userPattern)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao buscar mensagens: %v", err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		// Struct MsgInfo define a estrutura de dados e mapeamento correspondente
		type MsgInfo struct {
			ID          int    `json:"id"`
			SenderName  string `json:"sender_name"`
			SenderJID   string `json:"sender_jid"`
			Text        string `json:"text"`
			Type        string `json:"type"`
			IsFromMe    bool   `json:"is_from_me"`
			Timestamp   string `json:"timestamp"`
			MediaURL    string `json:"media_url,omitempty"`
			ReplyToName string `json:"reply_to_name,omitempty"`
			ReplyToText string `json:"reply_to_text,omitempty"`
			WAMessageID string `json:"wa_message_id,omitempty"`
		}

		messages := []MsgInfo{}
		for rows.Next() {
			var m MsgInfo
			var rawTime string
			var isFromMeInt int
			if err := rows.Scan(&m.ID, &m.SenderName, &m.SenderJID, &m.Text, &m.Type, &isFromMeInt, &rawTime, &m.MediaURL, &m.ReplyToName, &m.ReplyToText, &m.WAMessageID); err == nil {
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

	// API PARA APAGAR CONVERSA (Protegida)
	http.HandleFunc("/api/chats/delete", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "DELETE" && r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
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

		// Remove todas as mensagens do banco
		_, err := webDB.Exec("DELETE FROM chat_messages WHERE chat_jid = ?", jid)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao apagar chat: %v", err), http.StatusInternalServerError)
			return
		}

		// Reseta o estado do bot do usuário
		userJIDStr := NormalizePhoneLocal(jid)
		state.Mu.Lock()
		delete(state.Users, userJIDStr)
		state.Mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Conversa apagada com sucesso"})
	})

	// API PARA FINALIZAR ATENDIMENTO / REATIVAR BOT (Protegida)
	http.HandleFunc("/api/chats/close", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "POST" && r.Method != "DELETE" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		jid := r.URL.Query().Get("jid")
		if jid == "" {
			http.Error(w, "JID é obrigatório", http.StatusBadRequest)
			return
		}

		targetJID, err := types.ParseJID(jid)
		if err != nil {
			http.Error(w, "JID inválido", http.StatusBadRequest)
			return
		}

		userNumber := NormalizePhoneLocal(targetJID.User)

		state.Mu.Lock()
		uState, exists := state.Users[userNumber]
		if exists {
			// Reseta o passo para voltar ao bot
			uState.Step = -1
			uState.LastGreetingTime = time.Now().Add(-15 * time.Minute) // permite saudação imediata
		}

		// Se era o usuário ativo do live chat, libera
		if state.ActiveLiveChatUser == targetJID.String() {
			state.ActiveLiveChatUser = ""
			state.ActiveAgentName = ""
		}

		// Também remove da fila se estivesse nela
		for i, uFull := range state.LiveChatQueue {
			if uJID, _ := types.ParseJID(uFull); NormalizePhoneLocal(uJID.User) == userNumber {
				state.LiveChatQueue = append(state.LiveChatQueue[:i], state.LiveChatQueue[i+1:]...)
				break
			}
		}
		state.Mu.Unlock()

		ClientMu.Lock()
		client := GlobalClient
		ClientMu.Unlock()

		// Envia mensagem de encerramento para o usuário informando que o bot voltou
		if client != nil && client.IsConnected() {
			sendTextMessage(context.Background(), client, targetJID, "🤖 *Atendimento finalizado.* O bot automático foi reativado para esta conversa!")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Atendimento finalizado com sucesso"})
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
			JID         string `json:"jid"`
			Text        string `json:"text"`
			ReplyToName string `json:"reply_to_name"`
			ReplyToText string `json:"reply_to_text"`
			QuotedID    string `json:"quoted_id"`
			QuotedJID   string `json:"quoted_jid"`
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

		// Coloca o usuário em chat ao vivo/pausa o bot automático
		userNumber := NormalizePhoneLocal(targetJID.User)
		state.Mu.Lock()
		uState, exists := state.Users[userNumber]
		if !exists {
			uState = &state.UserState{Step: -1, LastGreetingTime: time.Now().Add(-15 * time.Minute)}
			state.Users[userNumber] = uState
		}
		uState.Step = 100

		// Define como o usuário ativo do Chat ao Vivo se não houver outro ativo
		if state.ActiveLiveChatUser == "" {
			state.ActiveLiveChatUser = targetJID.String()
		}
		state.Mu.Unlock()

		if req.ReplyToText != "" {
			qJID := req.QuotedJID
			if qJID == "" {
				qJID = targetJID.String()
			}
			sendQuotedTextMessage(context.Background(), client, targetJID, req.Text, qJID, req.ReplyToText, req.QuotedID)
		} else {
			sendTextMessage(context.Background(), client, targetJID, req.Text)
		}

		// Grava as informações da resposta citada na última mensagem enviada
		if webDB != nil && req.ReplyToText != "" {
			_, _ = webDB.Exec(
				"UPDATE chat_messages SET reply_to_name = ?, reply_to_text = ? WHERE id = (SELECT MAX(id) FROM chat_messages WHERE chat_jid = ? AND is_from_me = 1)",
				req.ReplyToName, req.ReplyToText, targetJID.String(),
			)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// API PARA ENVIAR IMAGENS E ARQUIVOS DO PAINEL WEB PARA O WHATSAPP (Protegida)
	http.HandleFunc("/api/chats/send-media", func(w http.ResponseWriter, r *http.Request) {
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

		// Limite de 20MB para upload
		if err := r.ParseMultipartForm(20 << 20); err != nil {
			http.Error(w, "Falha ao processar arquivo enviado", http.StatusBadRequest)
			return
		}

		jidStr := r.FormValue("jid")
		caption := r.FormValue("caption")
		replyToName := r.FormValue("reply_to_name")
		replyToText := r.FormValue("reply_to_text")
		quotedID := r.FormValue("quoted_id")
		quotedJID := r.FormValue("quoted_jid")

		if jidStr == "" {
			http.Error(w, "JID é obrigatório", http.StatusBadRequest)
			return
		}

		targetJID, err := types.ParseJID(jidStr)
		if err != nil {
			http.Error(w, "JID inválido", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Nenhum arquivo enviado", http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileBytes, err := io.ReadAll(file)
		if err != nil || len(fileBytes) == 0 {
			http.Error(w, "Arquivo vazio ou ilegível", http.StatusBadRequest)
			return
		}

		mimeType := header.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = http.DetectContentType(fileBytes)
		}

		// Converte os bytes da mídia diretamente para Base64 Data URL (sem salvar no disco do servidor)
		webMediaURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(fileBytes))

		// Coloca o usuário em chat ao vivo/pausa o bot automático
		userNumber := NormalizePhoneLocal(targetJID.User)
		state.Mu.Lock()
		uState, exists := state.Users[userNumber]
		if !exists {
			uState = &state.UserState{Step: -1, LastGreetingTime: time.Now().Add(-15 * time.Minute)}
			state.Users[userNumber] = uState
		}
		uState.Step = 100
		if state.ActiveLiveChatUser == "" {
			state.ActiveLiveChatUser = targetJID.String()
		}
		state.Mu.Unlock()

		// Prepara envio no whatsmeow
		ctx := context.Background()
		var msg *waE2E.Message
		msgType := "image"

		var contextInfo *waE2E.ContextInfo
		if replyToText != "" {
			qJID := quotedJID
			if qJID == "" {
				qJID = targetJID.String()
			}
			if pJID, errP := types.ParseJID(qJID); errP == nil {
				qJID = pJID.ToNonAD().String()
			}
			contextInfo = &waE2E.ContextInfo{
				Participant: proto.String(qJID),
				QuotedMessage: &waE2E.Message{
					Conversation: proto.String(replyToText),
				},
			}
			if quotedID != "" {
				contextInfo.StanzaID = proto.String(quotedID)
			}
		}

		lowerFilename := strings.ToLower(header.Filename)
		isAudio := strings.HasPrefix(mimeType, "audio/") ||
			strings.Contains(mimeType, "ogg") ||
			strings.Contains(mimeType, "webm") ||
			strings.HasPrefix(lowerFilename, "audio") ||
			strings.HasPrefix(lowerFilename, "voice") ||
			strings.HasSuffix(lowerFilename, ".ogg") ||
			strings.HasSuffix(lowerFilename, ".mp3") ||
			strings.HasSuffix(lowerFilename, ".m4a") ||
			strings.HasSuffix(lowerFilename, ".wav") ||
			strings.HasSuffix(lowerFilename, ".webm")

		if strings.HasPrefix(mimeType, "image/") {
			uploadResp, errUp := client.Upload(ctx, fileBytes, whatsmeow.MediaImage)
			if errUp != nil {
				http.Error(w, fmt.Sprintf("Falha ao enviar imagem ao WhatsApp: %v", errUp), http.StatusInternalServerError)
				return
			}

			imgMsg := &waE2E.ImageMessage{
				URL:           proto.String(uploadResp.URL),
				DirectPath:    proto.String(uploadResp.DirectPath),
				MediaKey:      uploadResp.MediaKey,
				Mimetype:      proto.String(mimeType),
				FileSHA256:    uploadResp.FileSHA256,
				FileEncSHA256: uploadResp.FileEncSHA256,
				FileLength:    proto.Uint64(uint64(len(fileBytes))),
				ContextInfo:   contextInfo,
			}
			if caption != "" {
				imgMsg.Caption = proto.String(caption)
			}
			msg = &waE2E.Message{ImageMessage: imgMsg}
		} else if isAudio {
			msgType = "audio"
			audioMime := "audio/ogg; codecs=opus"
			if strings.HasSuffix(lowerFilename, ".mp3") {
				audioMime = "audio/mp3"
			} else if strings.HasSuffix(lowerFilename, ".m4a") {
				audioMime = "audio/mp4"
			}

			uploadResp, errUp := client.Upload(ctx, fileBytes, whatsmeow.MediaAudio)
			if errUp != nil {
				http.Error(w, fmt.Sprintf("Falha ao enviar áudio ao WhatsApp: %v", errUp), http.StatusInternalServerError)
				return
			}

			audioMsg := &waE2E.AudioMessage{
				URL:           proto.String(uploadResp.URL),
				DirectPath:    proto.String(uploadResp.DirectPath),
				MediaKey:      uploadResp.MediaKey,
				Mimetype:      proto.String(audioMime),
				FileSHA256:    uploadResp.FileSHA256,
				FileEncSHA256: uploadResp.FileEncSHA256,
				FileLength:    proto.Uint64(uint64(len(fileBytes))),
				PTT:           proto.Bool(true),
				ContextInfo:   contextInfo,
			}
			msg = &waE2E.Message{AudioMessage: audioMsg}
		} else {
			msgType = "document"
			uploadResp, errUp := client.Upload(ctx, fileBytes, whatsmeow.MediaDocument)
			if errUp != nil {
				http.Error(w, fmt.Sprintf("Falha ao enviar documento ao WhatsApp: %v", errUp), http.StatusInternalServerError)
				return
			}

			docMsg := &waE2E.DocumentMessage{
				URL:           proto.String(uploadResp.URL),
				DirectPath:    proto.String(uploadResp.DirectPath),
				MediaKey:      uploadResp.MediaKey,
				Mimetype:      proto.String(mimeType),
				FileName:      proto.String(header.Filename),
				FileSHA256:    uploadResp.FileSHA256,
				FileEncSHA256: uploadResp.FileEncSHA256,
				FileLength:    proto.Uint64(uint64(len(fileBytes))),
				ContextInfo:   contextInfo,
			}
			if caption != "" {
				docMsg.Caption = proto.String(caption)
			}
			msg = &waE2E.Message{DocumentMessage: docMsg}
		}

		resp, errSend := sendMessage(ctx, client, targetJID, msg)
		if errSend != nil {
			http.Error(w, fmt.Sprintf("Erro ao entregar mídia: %v", errSend), http.StatusInternalServerError)
			return
		}

		// Atualiza o registro no webDB com media_url, reply_to e wa_message_id
		if webDB != nil {
			senderName := "GLPI-BOT (Suporte)"
			if sess, ok := getUserSession(r); ok && sess.Name != "" {
				senderName = sess.Name + " (Suporte)"
			}

			msgText := caption
			if msgText == "" {
				if msgType == "image" {
					msgText = "[Imagem]"
				} else if msgType == "audio" {
					msgText = "[Áudio]"
				} else {
					msgText = "[Documento] " + header.Filename
				}
			}

			_, _ = webDB.Exec(
				"INSERT INTO chat_messages (chat_jid, sender_name, sender_jid, message_text, message_type, is_from_me, media_url, reply_to_name, reply_to_text, wa_message_id) VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, ?)",
				targetJID.String(), senderName, "", msgText, msgType, webMediaURL, replyToName, replyToText, resp.ID,
			)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "media_url": webMediaURL})
	})

	// API PARA LISTAR OS ATENDENTES/TÉCNICOS (Protegida)
	http.HandleFunc("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "GET" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		agents := obterAtendentesSuporte()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agents)
	})

	// API PARA ASSUMIR UM ATENDIMENTO PELO PAINEL (Protegida)
	http.HandleFunc("/api/chats/assume", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			JID   string `json:"jid"`
			Agent string `json:"agent"`
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

		userNumber := NormalizePhoneLocal(targetJID.User)

		state.Mu.Lock()
		uState, exists := state.Users[userNumber]
		if !exists {
			uState = &state.UserState{Step: -1, LastGreetingTime: time.Now().Add(-15 * time.Minute)}
			state.Users[userNumber] = uState
		}
		uState.Step = 100

		state.ActiveAgentName = req.Agent
		state.ActiveLiveChatUser = targetJID.String()

		// Remove da fila de espera se estiver nela
		for i, uFull := range state.LiveChatQueue {
			if uJID, _ := types.ParseJID(uFull); NormalizePhoneLocal(uJID.User) == userNumber {
				state.LiveChatQueue = append(state.LiveChatQueue[:i], state.LiveChatQueue[i+1:]...)
				break
			}
		}

		nomeUsuario := state.Names[userNumber]
		if nomeUsuario == "" {
			nomeUsuario = userNumber
		}
		state.Mu.Unlock()

		ClientMu.Lock()
		client := GlobalClient
		ClientMu.Unlock()

		if client != nil && client.IsConnected() {
			// Envia a mensagem de suporte assumido para o usuário
			msgAssumido := formatarMensagem(config.GetConfig().MsgSuporteAssumido, map[string]string{"agente": req.Agent})
			sendTextMessage(context.Background(), client, targetJID, msgAssumido)

			// Notifica o grupo/número de suporte
			supportJID := types.NewJID(getSupportNumber(), types.DefaultUserServer)
			sendTextMessage(context.Background(), client, supportJID, fmt.Sprintf(
				"✅ O atendimento de *%s* foi assumido via Painel por *%s*.",
				nomeUsuario, req.Agent,
			))
		} else {
			// Se o bot estiver desconectado, podemos pelo menos inserir a mensagem de sistema no banco para que o painel mostre
			if webDB != nil {
				msgAssumido := formatarMensagem(config.GetConfig().MsgSuporteAssumido, map[string]string{"agente": req.Agent})
				_, _ = webDB.Exec(
					"INSERT INTO chat_messages (chat_jid, sender_name, sender_jid, message_text, message_type, is_from_me) VALUES (?, ?, ?, ?, ?, 1)",
					targetJID.String(), "GLPI-BOT (Bot)", "", msgAssumido, "text",
				)
			}
		}

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

// Função getLocalIPs executa a regra de negócio/rotina correspondente
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

// Função getUptime executa a regra de negócio/rotina correspondente
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

// Função triggerManualQRFlow executa a regra de negócio/rotina correspondente
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
