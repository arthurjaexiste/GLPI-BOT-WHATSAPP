package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"bot-glpi/internal/config"
	"bot-glpi/internal/database"
	"bot-glpi/internal/whatsapp"
)

// HandleLogsAPI retorna ou permite download do arquivo de logs
func HandleLogsAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

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
	}
}

// HandleRecentTicketsAPI lista os chamados abertos recentemente
func HandleRecentTicketsAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if database.DB == nil {
			http.Error(w, "Banco de dados não disponível", http.StatusInternalServerError)
			return
		}

		rows, err := database.DB.Query("SELECT ticket_id, title, requester, created_at FROM tickets_history ORDER BY id DESC LIMIT 5")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao buscar histórico: %v", err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type TicketItem struct {
			ID        string `json:"id"`
			Title     string `json:"title"`
			Requester string `json:"requester"`
			CreatedAt string `json:"created_at"`
		}

		tickets := []TicketItem{}
		for rows.Next() {
			var t TicketItem
			var rawTime string
			if err := rows.Scan(&t.ID, &t.Title, &t.Requester, &rawTime); err == nil {
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
	}
}

// HandleConfigAPI consulta e persiste configurações gerais da plataforma
func HandleConfigAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
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
	}
}

// HandleTestSMTPAPI valida credenciais e dispara e-mail de teste
func HandleTestSMTPAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
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

		err := whatsapp.SendSMTPTestEmail(testCfg)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "message": err.Error()})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// HandleFlowAPI obtém e atualiza o fluxo interativo de navegação
func HandleFlowAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			flow := whatsapp.GetFlowConfig()
			json.NewEncoder(w).Encode(flow)
			return
		}

		if r.Method == "POST" {
			var newFlow whatsapp.FlowNode
			if err := json.NewDecoder(r.Body).Decode(&newFlow); err != nil {
				http.Error(w, "JSON inválido", http.StatusBadRequest)
				return
			}

			if err := whatsapp.SaveFlowConfig(newFlow); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
			return
		}

		http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
	}
}

// HandleRestartAPI solicita reinicialização graciosa do processo
func HandleRestartAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
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
	}
}
