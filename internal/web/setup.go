package web

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"bot-glpi/internal/database"
)

// HandleSetup gerencia a rota de Primeiro Acesso para criação do administrador inicial
func HandleSetup(webDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if database.HasAdminUser() {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		if r.Method == "GET" {
			tmpl, err := template.ParseFiles(filepath.Join(webDir, "setup.html"))
			if err != nil {
				http.Error(w, fmt.Sprintf("Erro ao carregar tela de configuração: %v", err), http.StatusInternalServerError)
				return
			}
			tmpl.Execute(w, map[string]string{})
			return
		}

		if r.Method == "POST" {
			if database.DB == nil {
				http.Error(w, "Erro interno: banco de dados indisponível.", http.StatusInternalServerError)
				return
			}

			r.ParseForm()
			name := strings.TrimSpace(r.FormValue("name"))
			username := strings.TrimSpace(r.FormValue("username"))
			email := strings.TrimSpace(r.FormValue("email"))
			password := r.FormValue("password")
			confirmPassword := r.FormValue("confirm_password")

			renderError := func(errMsg string) {
				tmpl, err := template.ParseFiles(filepath.Join(webDir, "setup.html"))
				if err != nil {
					http.Error(w, fmt.Sprintf("Erro ao carregar tela de configuração: %v", err), http.StatusInternalServerError)
					return
				}
				tmpl.Execute(w, map[string]string{
					"Error":    errMsg,
					"Name":     name,
					"Username": username,
					"Email":    email,
				})
			}

			if name == "" || username == "" || email == "" || password == "" {
				renderError("Todos os campos são obrigatórios.")
				return
			}

			if strings.ContainsAny(username, " \t\n\r") {
				renderError("O nome de usuário não pode conter espaços em branco.")
				return
			}

			if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
				renderError("Informe um endereço de e-mail válido.")
				return
			}

			if len(password) < 6 {
				renderError("A senha deve ter no mínimo 6 caracteres.")
				return
			}

			if password != confirmPassword {
				renderError("A confirmação de senha não confere com a senha digitada.")
				return
			}

			hash, err := database.HashPassword(password)
			if err != nil {
				renderError("Erro ao processar senha de segurança.")
				return
			}

			res, err := database.DB.Exec("INSERT INTO users (username, password, name, email, role, enabled) VALUES (?, ?, ?, ?, 'admin', 1)", username, hash, name, email)
			if err != nil {
				if strings.Contains(err.Error(), "UNIQUE") {
					renderError("Este nome de usuário já está em uso.")
					return
				}
				renderError(fmt.Sprintf("Erro ao salvar administrador: %v", err))
				return
			}

			newID, _ := res.LastInsertId()
			fmt.Printf("✅ Administrador inicial configurado com sucesso! (Usuário: %s, Email: %s)\n", username, email)

			token := GenerateToken()
			userSess := UserSession{
				UserID:   int(newID),
				Username: username,
				Name:     name,
				Role:     "admin",
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
		}
	}
}
