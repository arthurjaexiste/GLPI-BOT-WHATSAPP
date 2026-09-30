package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bot-glpi/internal/database"
	"bot-glpi/internal/glpi"
)

// UserSession armazena as informações do usuário autenticado no cookie de sessão
type UserSession struct {
	UserID   int       `json:"user_id"`
	Username string    `json:"username"`
	Name     string    `json:"name"`
	Role     string    `json:"role"`
	Expiry   time.Time `json:"expiry"`
}

var (
	sessions   = make(map[string]UserSession)
	sessionsMu sync.Mutex
)

// GenerateToken cria um token hexadecimal seguro para a sessão
func GenerateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetUserSession recupera a sessão do usuário com base no cookie HTTP
func GetUserSession(r *http.Request) (UserSession, bool) {
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

// IsAuthenticated valida se o usuário possui sessão ativa e válida
func IsAuthenticated(r *http.Request) bool {
	_, ok := GetUserSession(r)
	return ok
}

// HandleLogin gerencia o login tradicional e validações de credenciais
func HandleLogin(webDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !database.HasAdminUser() {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}

		if r.Method == "GET" {
			if IsAuthenticated(r) {
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			tmpl, err := template.ParseFiles(filepath.Join(webDir, "login.html"))
			if err != nil {
				http.Error(w, fmt.Sprintf("Erro ao carregar login: %v", err), http.StatusInternalServerError)
				return
			}
			tmpl.Execute(w, map[string]string{})
			return
		}

		if r.Method == "POST" {
			if database.DB == nil {
				fmt.Println("🚨 ERRO CRÍTICO: database.DB é nil no login!")
				http.Error(w, "Erro interno: banco de dados indisponível.", http.StatusInternalServerError)
				return
			}

			r.ParseForm()
			username := strings.TrimSpace(r.FormValue("username"))
			password := r.FormValue("password")

			var id, glpiID, enabled int
			var dbPass, name, role string
			err := database.DB.QueryRow("SELECT id, COALESCE(glpi_id, 0), COALESCE(password, ''), COALESCE(name, 'Usuário'), COALESCE(role, 'operator'), COALESCE(enabled, 1) FROM users WHERE LOWER(username) = LOWER(?)", username).Scan(&id, &glpiID, &dbPass, &name, &role, &enabled)

			valid := false
			loginErrorMsg := "Usuário ou senha inválidos."

			if err == nil {
				if enabled == 0 {
					tmpl, errTmpl := template.ParseFiles(filepath.Join(webDir, "login.html"))
					if errTmpl != nil {
						http.Error(w, fmt.Sprintf("Erro ao carregar login: %v", errTmpl), http.StatusInternalServerError)
						return
					}
					tmpl.Execute(w, map[string]string{"Error": "Acesso desativado para este usuário. Entre em contato com o administrador."})
					return
				}

				if dbPass != "" && dbPass != "glpi_user" {
					if database.CheckPasswordHash(password, dbPass) {
						valid = true
					} else if dbPass == password {
						valid = true
						if hash, errHash := database.HashPassword(password); errHash == nil {
							_, _ = database.DB.Exec("UPDATE users SET password = ? WHERE id = ?", hash, id)
						}
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
						var newID int64
						res, errIns := database.DB.Exec("INSERT INTO users (username, password, name, role, enabled) VALUES (?, 'glpi_user', ?, 'operator', 1)", username, username)
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

			if valid {
				token := GenerateToken()
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
	}
}

// HandleLogout efetua o logout do usuário e invalida a sessão
func HandleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
	}
}

// HandleApiMe retorna os dados do usuário autenticado atualmente
func HandleApiMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetUserSession(r)
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
	}
}
