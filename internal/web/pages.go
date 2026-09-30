package web

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	"bot-glpi/internal/database"
)

// HandleIndex renderiza o dashboard principal
func HandleIndex(webDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if !database.HasAdminUser() {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		if !IsAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		tmpl, err := template.ParseFiles(filepath.Join(webDir, "index.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar a interface: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	}
}

// HandleConfigPage renderiza a tela de configurações (exclusivo para administradores)
func HandleConfigPage(webDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetUserSession(r)
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
	}
}

// HandleMessagesPage renderiza a tela de mensagens personalizadas do bot
func HandleMessagesPage(webDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetUserSession(r)
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
	}
}

// HandleLogsPage renderiza a tela de logs do sistema
func HandleLogsPage(webDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetUserSession(r)
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
	}
}

// HandleChatsPage renderiza o painel de atendimento e conversas em tempo real
func HandleChatsPage(webDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		tmpl, err := template.ParseFiles(filepath.Join(webDir, "chats.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar painel de chats: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	}
}

// HandleFlowPage renderiza o editor do fluxo de atendimento
func HandleFlowPage(webDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetUserSession(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if session.Role != "admin" {
			http.Redirect(w, r, "/chats", http.StatusSeeOther)
			return
		}
		tmpl, err := template.ParseFiles(filepath.Join(webDir, "flow.html"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao carregar o construtor de fluxo: %v", err), http.StatusInternalServerError)
			return
		}
		tmpl.Execute(w, nil)
	}
}
