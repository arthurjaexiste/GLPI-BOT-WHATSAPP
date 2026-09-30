package web

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
)

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

// StartWebServer inicializa o servidor HTTP e todas as rotas da interface web
func StartWebServer() {
	porta := "0.0.0.0:33090"
	basePath := getWebDir()
	webDir := filepath.Join(basePath, "web")
	staticDir := filepath.Join(basePath, "static")

	fs := http.FileServer(http.Dir(staticDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// ─── Autenticação e Primeiro Acesso ──────────────────────────────────────
	http.HandleFunc("/setup", HandleSetup(webDir))
	http.HandleFunc("/login", HandleLogin(webDir))
	http.HandleFunc("/logout", HandleLogout())
	http.HandleFunc("/api/me", HandleApiMe())

	// ─── Páginas Principais (Views) ──────────────────────────────────────────
	http.HandleFunc("/", HandleIndex(webDir))
	http.HandleFunc("/config", HandleConfigPage(webDir))
	http.HandleFunc("/messages", HandleMessagesPage(webDir))
	http.HandleFunc("/logs", HandleLogsPage(webDir))
	http.HandleFunc("/chats", HandleChatsPage(webDir))
	http.HandleFunc("/flow", HandleFlowPage(webDir))

	// ─── Gestão de Usuários ──────────────────────────────────────────────────
	http.HandleFunc("/api/users", HandleUsersAPI())
	http.HandleFunc("/api/glpi/users", HandleGLPIUsersAPI())
	http.HandleFunc("/api/users/toggle-access", HandleToggleAccessAPI())
	http.HandleFunc("/api/users/delete", HandleDeleteUserAPI())
	http.HandleFunc("/api/change-password", HandleChangePasswordAPI())

	// ─── WhatsApp & Mensagens ────────────────────────────────────────────────
	http.HandleFunc("/api/whatsapp/connect", HandleWhatsAppConnect())
	http.HandleFunc("/api/whatsapp/logout", HandleWhatsAppLogout())
	http.HandleFunc("/api/chats", HandleChatsAPI())
	http.HandleFunc("/api/chats/avatar", HandleChatAvatarAPI())
	http.HandleFunc("/api/chats/messages", HandleChatMessagesAPI())
	http.HandleFunc("/api/chats/delete", HandleChatDeleteAPI())
	http.HandleFunc("/api/chats/close", HandleChatCloseAPI())
	http.HandleFunc("/api/chats/send", HandleChatSendAPI())
	http.HandleFunc("/api/chats/send-media", HandleChatSendMediaAPI())
	http.HandleFunc("/api/agents", HandleAgentsAPI())
	http.HandleFunc("/api/chats/assume", HandleChatAssumeAPI())
	http.HandleFunc("/api/status", HandleStatusAPI())
	http.HandleFunc("/api/webhook/glpi", HandleWebhookGLPIAPI())

	// ─── Configurações e Logs ────────────────────────────────────────────────
	http.HandleFunc("/api/logs", HandleLogsAPI())
	http.HandleFunc("/api/tickets/recent", HandleRecentTicketsAPI())
	http.HandleFunc("/api/config", HandleConfigAPI())
	http.HandleFunc("/api/config/test-smtp", HandleTestSMTPAPI())
	http.HandleFunc("/api/flow", HandleFlowAPI())
	http.HandleFunc("/api/restart", HandleRestartAPI())

	fmt.Printf("🌐 Painel Web protegido rodando em:\n")
	fmt.Printf("   - http://localhost:33090\n")
	for _, ip := range getLocalIPs() {
		fmt.Printf("   - http://%s:33090\n", ip)
	}

	if err := http.ListenAndServe(porta, &recoveryHandler{handler: http.DefaultServeMux}); err != nil {
		fmt.Printf("🚨 Erro fatal no servidor web: %v\n", err)
	}
}
