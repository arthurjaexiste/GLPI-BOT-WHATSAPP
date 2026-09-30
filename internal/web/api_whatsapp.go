package web

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bot-glpi/internal/config"
	"bot-glpi/internal/database"
	"bot-glpi/internal/state"
	"bot-glpi/internal/whatsapp"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

var startTime = time.Now()

// HandleWhatsAppConnect tenta reconectar ao WhatsApp
func HandleWhatsAppConnect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		whatsapp.ClientMu.Lock()
		client := whatsapp.GlobalClient
		whatsapp.ClientMu.Unlock()

		if client == nil {
			http.Error(w, "Cliente não inicializado", http.StatusInternalServerError)
			return
		}

		go func() {
			fmt.Println("🔄 Tentando reconectar ao WhatsApp via solicitação web...")
			if client.Store.ID == nil {
				whatsapp.TriggerManualQRFlow()
			} else {
				_ = client.Connect()
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Tentando conectar..."})
	}
}

// HandleWhatsAppLogout desconecta e reseta a sessão do WhatsApp gerando novo QR Code
func HandleWhatsAppLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		whatsapp.ClientMu.Lock()
		client := whatsapp.GlobalClient
		whatsapp.ClientMu.Unlock()

		if client == nil {
			http.Error(w, "Cliente não inicializado", http.StatusInternalServerError)
			return
		}

		go func() {
			fmt.Println("⚠️ Solicitado logout/reset de sessão via painel web. Desconectando...")
			if client.IsConnected() {
				err := client.Logout(context.Background())
				if err != nil {
					whatsapp.TriggerManualQRFlow()
				}
			} else {
				whatsapp.TriggerManualQRFlow()
			}
		}()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Desconectando e iniciando novo QR Code..."})
	}
}

// HandleChatsAPI lista as conversas ativas
func HandleChatsAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if database.DB == nil {
			http.Error(w, "Banco de dados não disponível", http.StatusInternalServerError)
			return
		}

		query := `
			SELECT c.chat_jid, c.sender_name, c.message_text, c.timestamp
			FROM chat_messages c
			INNER JOIN (
				SELECT chat_jid, MAX(id) as max_id
				FROM chat_messages
				WHERE chat_jid NOT LIKE '%@g.us%' AND chat_jid NOT LIKE '%g.us%' AND chat_jid NOT LIKE '120363%'
				GROUP BY chat_jid
			) m ON c.id = m.max_id
			WHERE c.chat_jid NOT LIKE '%@g.us%' AND c.chat_jid NOT LIKE '%g.us%' AND c.chat_jid NOT LIKE '120363%'
			ORDER BY c.timestamp DESC
		`
		rows, err := database.DB.Query(query)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao buscar chats: %v", err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		whatsapp.ClientMu.Lock()
		client := whatsapp.GlobalClient
		whatsapp.ClientMu.Unlock()

		botJID := ""
		if client != nil && client.Store != nil && client.Store.ID != nil {
			botJID = client.Store.ID.User + "@" + client.Store.ID.Server
		}

		type ChatInfo struct {
			JID        string `json:"jid"`
			Name       string `json:"name"`
			LastMsg    string `json:"last_message"`
			Timestamp  string `json:"timestamp"`
			UserStatus string `json:"status"`
			AgentName  string `json:"agent_name,omitempty"`
		}

		chats := []ChatInfo{}
		for rows.Next() {
			var c ChatInfo
			var rawTime string
			if err := rows.Scan(&c.JID, &c.Name, &c.LastMsg, &rawTime); err == nil {
				if botJID != "" && (c.JID == botJID || strings.Split(c.JID, "@")[0] == strings.Split(botJID, "@")[0]) {
					continue
				}
				supportNum := whatsapp.GetSupportNumber()
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
				userJIDStr := whatsapp.NormalizePhoneLocal(c.JID)
				state.Mu.Lock()
				if uState, exists := state.Users[userJIDStr]; exists {
					if uState.Step == 100 {
						c.UserStatus = "live_chat"
						if whatsapp.NormalizePhoneLocal(state.ActiveLiveChatUser) == userJIDStr && state.ActiveAgentName != "" {
							c.AgentName = state.ActiveAgentName
						}
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
	}
}

// HandleChatAvatarAPI busca imagem de avatar do WhatsApp ou usa placeholder
func HandleChatAvatarAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
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

		whatsapp.ClientMu.Lock()
		client := whatsapp.GlobalClient
		whatsapp.ClientMu.Unlock()

		if client == nil || !client.IsConnected() {
			http.Redirect(w, r, fallbackURL, http.StatusTemporaryRedirect)
			return
		}

		targetJID, err := types.ParseJID(jidParam)
		if err != nil {
			http.Redirect(w, r, fallbackURL, http.StatusTemporaryRedirect)
			return
		}

		avatarInfo, err := client.GetProfilePictureInfo(r.Context(), targetJID, &whatsmeow.GetProfilePictureParams{})
		if err != nil || avatarInfo == nil || avatarInfo.URL == "" {
			http.Redirect(w, r, fallbackURL, http.StatusTemporaryRedirect)
			return
		}

		http.Redirect(w, r, avatarInfo.URL, http.StatusTemporaryRedirect)
	}
}

// HandleChatMessagesAPI recupera o histórico de mensagens de uma conversa
func HandleChatMessagesAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if database.DB == nil {
			http.Error(w, "Banco de dados não disponível", http.StatusInternalServerError)
			return
		}

		rawJID := r.URL.Query().Get("jid")
		if rawJID == "" {
			http.Error(w, "JID é obrigatório", http.StatusBadRequest)
			return
		}

		cleanJID := whatsapp.CleanJIDString(rawJID)
		userPhone := whatsapp.NormalizePhoneLocal(cleanJID)
		userPattern := "%" + userPhone + "%"

		rows, err := database.DB.Query(`
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
	}
}

// HandleChatDeleteAPI apaga histórico de mensagens de uma conversa
func HandleChatDeleteAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "DELETE" && r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		if database.DB == nil {
			http.Error(w, "Banco de dados não disponível", http.StatusInternalServerError)
			return
		}

		jid := r.URL.Query().Get("jid")
		if jid == "" {
			http.Error(w, "JID é obrigatório", http.StatusBadRequest)
			return
		}

		cleanJID := whatsapp.CleanJIDString(jid)
		userNum := whatsapp.NormalizePhoneLocal(cleanJID)
		userPattern := "%" + userNum + "%"

		_, err := database.DB.Exec("DELETE FROM chat_messages WHERE chat_jid = ? OR chat_jid = ? OR chat_jid LIKE ?", cleanJID, jid, userPattern)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao apagar chat: %v", err), http.StatusInternalServerError)
			return
		}

		state.Mu.Lock()
		state.RemoveAgentForUser(userNum)
		newQueue := []string{}
		for _, q := range state.LiveChatQueue {
			if whatsapp.NormalizePhoneLocal(q) != userNum {
				newQueue = append(newQueue, q)
			}
		}
		state.LiveChatQueue = newQueue
		delete(state.Users, userNum)
		state.Mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Conversa apagada com sucesso"})
	}
}

// HandleChatCloseAPI encerra atendimento ao vivo e reativa o bot
func HandleChatCloseAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
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

		userNumber := whatsapp.NormalizePhoneLocal(targetJID.User)

		agentName := ""
		isAdmin := false
		if sess, ok := GetUserSession(r); ok {
			isAdmin = (sess.Role == "admin")
			if sess.Name != "" {
				agentName = sess.Name
			} else if sess.Username != "" {
				agentName = sess.Username
			}
		}

		state.Mu.Lock()
		assignedAgent := state.GetAgentForUser(userNumber)
		if !isAdmin && assignedAgent != "" && agentName != "" && assignedAgent != agentName {
			state.Mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": fmt.Sprintf("Apenas o técnico *%s* ou um Administrador pode finalizar este atendimento.", assignedAgent),
				"agent":   assignedAgent,
			})
			return
		}

		uState, exists := state.Users[userNumber]
		if exists {
			uState.Step = -1
			uState.LastGreetingTime = time.Now().Add(-15 * time.Minute)
		}

		state.RemoveAgentForUser(userNumber)

		for i, uFull := range state.LiveChatQueue {
			if uJID, _ := types.ParseJID(uFull); whatsapp.NormalizePhoneLocal(uJID.User) == userNumber {
				state.LiveChatQueue = append(state.LiveChatQueue[:i], state.LiveChatQueue[i+1:]...)
				break
			}
		}
		state.Mu.Unlock()

		whatsapp.ClientMu.Lock()
		client := whatsapp.GlobalClient
		whatsapp.ClientMu.Unlock()

		if client != nil && client.IsConnected() {
			whatsapp.SendTextMessage(context.Background(), client, targetJID, "🤖 *Atendimento finalizado.* O bot automático foi reativado para esta conversa!")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Atendimento finalizado com sucesso"})
	}
}

// HandleChatSendAPI envia mensagens do painel web para o WhatsApp
func HandleChatSendAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		whatsapp.ClientMu.Lock()
		client := whatsapp.GlobalClient
		whatsapp.ClientMu.Unlock()

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

		userNumber := whatsapp.NormalizePhoneLocal(targetJID.User)

		agentName := ""
		isAdmin := false
		if sess, ok := GetUserSession(r); ok {
			isAdmin = (sess.Role == "admin")
			if sess.Name != "" {
				agentName = sess.Name
			} else if sess.Username != "" {
				agentName = sess.Username
			}
		}
		if agentName == "" {
			agentName = "Suporte"
		}

		state.Mu.Lock()
		assignedAgent := state.GetAgentForUser(userNumber)
		uState, exists := state.Users[userNumber]
		isLiveChat := exists && uState.Step == 100

		if !isAdmin && isLiveChat && assignedAgent != "" && assignedAgent != agentName {
			state.Mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": fmt.Sprintf("Apenas o técnico *%s* ou um Administrador pode responder esta conversa.", assignedAgent),
				"agent":   assignedAgent,
			})
			return
		}

		if !exists {
			uState = &state.UserState{Step: -1, LastGreetingTime: time.Now().Add(-15 * time.Minute)}
			state.Users[userNumber] = uState
		}
		uState.Step = 100
		state.SetAgentForUser(userNumber, agentName)
		state.Mu.Unlock()

		waText := fmt.Sprintf("> 👨‍💻 *%s:*\n%s", agentName, req.Text)

		if req.ReplyToText != "" {
			qJID := req.QuotedJID
			if qJID == "" {
				qJID = targetJID.String()
			}
			whatsapp.SendQuotedTextMessage(context.Background(), client, targetJID, waText, qJID, req.ReplyToText, req.QuotedID)
		} else {
			whatsapp.SendTextMessage(context.Background(), client, targetJID, waText)
		}

		cleanChatJID := whatsapp.CleanJIDString(targetJID.String())
		if database.DB != nil {
			if req.ReplyToText != "" {
				_, _ = database.DB.Exec(
					"UPDATE chat_messages SET message_text = ?, sender_name = ?, reply_to_name = ?, reply_to_text = ? WHERE id = (SELECT MAX(id) FROM chat_messages WHERE (chat_jid = ? OR chat_jid = ?) AND is_from_me = 1)",
					req.Text, agentName, req.ReplyToName, req.ReplyToText, cleanChatJID, targetJID.String(),
				)
			} else {
				_, _ = database.DB.Exec(
					"UPDATE chat_messages SET message_text = ?, sender_name = ? WHERE id = (SELECT MAX(id) FROM chat_messages WHERE (chat_jid = ? OR chat_jid = ?) AND is_from_me = 1)",
					req.Text, agentName, cleanChatJID, targetJID.String(),
				)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// HandleChatSendMediaAPI envia imagens, áudios e anexos para o WhatsApp
func HandleChatSendMediaAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		whatsapp.ClientMu.Lock()
		client := whatsapp.GlobalClient
		whatsapp.ClientMu.Unlock()

		if client == nil || !client.IsConnected() {
			http.Error(w, "WhatsApp desconectado", http.StatusServiceUnavailable)
			return
		}

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

		webMediaURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(fileBytes))

		userNumber := whatsapp.NormalizePhoneLocal(targetJID.User)
		agentName := ""
		isAdmin := false
		if sess, ok := GetUserSession(r); ok {
			isAdmin = (sess.Role == "admin")
			if sess.Name != "" {
				agentName = sess.Name
			} else if sess.Username != "" {
				agentName = sess.Username
			}
		}
		if agentName == "" {
			agentName = "Suporte"
		}

		state.Mu.Lock()
		assignedAgent := state.GetAgentForUser(userNumber)
		uState, exists := state.Users[userNumber]
		isLiveChat := exists && uState.Step == 100

		if !isAdmin && isLiveChat && assignedAgent != "" && assignedAgent != agentName {
			state.Mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": fmt.Sprintf("Apenas o técnico *%s* ou um Administrador pode enviar mídias nesta conversa.", assignedAgent),
				"agent":   assignedAgent,
			})
			return
		}

		if !exists {
			uState = &state.UserState{Step: -1, LastGreetingTime: time.Now().Add(-15 * time.Minute)}
			state.Users[userNumber] = uState
		}
		uState.Step = 100
		state.SetAgentForUser(userNumber, agentName)
		state.Mu.Unlock()

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

		resp, errSend := whatsapp.SendMessage(ctx, client, targetJID, msg)
		if errSend != nil {
			http.Error(w, fmt.Sprintf("Erro ao entregar mídia: %v", errSend), http.StatusInternalServerError)
			return
		}

		if database.DB != nil {
			senderName := "GLPI-BOT (Suporte)"
			if sess, ok := GetUserSession(r); ok && sess.Name != "" {
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

			_, _ = database.DB.Exec(
				"INSERT INTO chat_messages (chat_jid, sender_name, sender_jid, message_text, message_type, is_from_me, media_url, reply_to_name, reply_to_text, wa_message_id) VALUES (?, ?, ?, ?, ?, 1, ?, ?, ?, ?)",
				targetJID.String(), senderName, "", msgText, msgType, webMediaURL, replyToName, replyToText, resp.ID,
			)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "media_url": webMediaURL})
	}
}

// HandleAgentsAPI retorna a lista de atendentes de suporte
func HandleAgentsAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "GET" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		agents := whatsapp.ObterAtendentesSuporte()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agents)
	}
}

// HandleChatAssumeAPI define um técnico para assumir a conversa
func HandleChatAssumeAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
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
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		targetJID, err := types.ParseJID(req.JID)
		if err != nil {
			http.Error(w, "JID inválido", http.StatusBadRequest)
			return
		}

		agentName := req.Agent
		isAdmin := false
		if sess, ok := GetUserSession(r); ok {
			isAdmin = (sess.Role == "admin")
			if sess.Name != "" {
				agentName = sess.Name
			} else if sess.Username != "" {
				agentName = sess.Username
			}
		}
		if agentName == "" {
			agentName = "Suporte"
		}

		userNumber := whatsapp.NormalizePhoneLocal(targetJID.User)

		state.Mu.Lock()
		assignedAgent := state.GetAgentForUser(userNumber)
		uState, exists := state.Users[userNumber]
		isLiveChat := exists && uState.Step == 100

		if !isAdmin && isLiveChat && assignedAgent != "" && assignedAgent != agentName {
			state.Mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": fmt.Sprintf("Esta conversa já está em atendimento pelo técnico: %s.", assignedAgent),
				"agent":   assignedAgent,
			})
			return
		}

		if !exists {
			uState = &state.UserState{Step: -1, LastGreetingTime: time.Now().Add(-15 * time.Minute)}
			state.Users[userNumber] = uState
		}
		uState.Step = 100

		state.ActiveAgentName = agentName
		state.ActiveLiveChatUser = targetJID.String()

		for i, uFull := range state.LiveChatQueue {
			if uJID, _ := types.ParseJID(uFull); whatsapp.NormalizePhoneLocal(uJID.User) == userNumber {
				state.LiveChatQueue = append(state.LiveChatQueue[:i], state.LiveChatQueue[i+1:]...)
				break
			}
		}

		nomeUsuario := state.Names[userNumber]
		if nomeUsuario == "" {
			nomeUsuario = userNumber
		}
		state.Mu.Unlock()

		whatsapp.ClientMu.Lock()
		client := whatsapp.GlobalClient
		whatsapp.ClientMu.Unlock()

		if client != nil && client.IsConnected() {
			msgAssumido := whatsapp.FormatarMensagem(config.GetConfig().MsgSuporteAssumido, map[string]string{"agente": agentName})
			whatsapp.SendTextMessage(context.Background(), client, targetJID, msgAssumido)

			supportJID := types.NewJID(whatsapp.GetSupportNumber(), types.DefaultUserServer)
			whatsapp.SendTextMessage(context.Background(), client, supportJID, fmt.Sprintf(
				"✅ O atendimento de *%s* foi assumido via Painel por *%s*.",
				nomeUsuario, agentName,
			))
		} else {
			if database.DB != nil {
				msgAssumido := whatsapp.FormatarMensagem(config.GetConfig().MsgSuporteAssumido, map[string]string{"agente": agentName})
				_, _ = database.DB.Exec(
					"INSERT INTO chat_messages (chat_jid, sender_name, sender_jid, message_text, message_type, is_from_me) VALUES (?, ?, ?, ?, ?, 1)",
					targetJID.String(), "GLPI-BOT (Bot)", "", msgAssumido, "text",
				)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

// HandleStatusAPI fornece status em tempo real do bot, conexão e QR Code
func HandleStatusAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := "waiting"
		if whatsapp.IsConnected {
			status = "connected"
		} else if whatsapp.CurrentQR != "" {
			status = "qr"
		}

		mainIP := r.Host
		if shost, _, err := net.SplitHostPort(r.Host); err == nil {
			mainIP = shost
		}
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
			"qr":       whatsapp.CurrentQR,
			"ip":       mainIP,
			"engine":   "Whatsmeow (Multi-Device)",
			"timezone": "America/Sao_Paulo (UTC-3)",
			"uptime":   getUptime(),
		})
	}
}

// HandleWebhookGLPIAPI recebe alertas de novos chamados abertos no GLPI
func HandleWebhookGLPIAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		finalTitle := payload.Title
		if finalTitle == "" {
			finalTitle = payload.Name
		}
		if finalTitle == "" {
			finalTitle = "Sem título"
		}

		finalRequester := payload.Requester
		if finalRequester == "" {
			finalRequester = payload.User
		}
		if finalRequester == "" {
			finalRequester = "E-mail Receiver"
		}

		finalSource := payload.Source
		if finalSource == "" {
			finalSource = payload.Origem
		}
		if finalSource == "" {
			finalSource = "E-mail"
		}

		numeroTI := config.GetConfig().TelefoneNotificacao
		if numeroTI == "" {
			http.Error(w, `{"error": "Telefone de notificação não configurado"}`, http.StatusPreconditionFailed)
			return
		}

		whatsapp.ClientMu.Lock()
		client := whatsapp.GlobalClient
		connected := whatsapp.IsConnected
		whatsapp.ClientMu.Unlock()

		if client == nil || !connected {
			http.Error(w, `{"error": "Bot de WhatsApp desconectado"}`, http.StatusServiceUnavailable)
			return
		}

		targetJID := types.NewJID(numeroTI, types.DefaultUserServer)
		linkTicket := whatsapp.ObterLinkTicketGLPI(finalID)
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

		if database.DB != nil {
			_, errDb := database.DB.Exec("INSERT INTO tickets_history (ticket_id, title, requester) VALUES (?, ?, ?)", finalID, finalTitle, finalRequester)
			if errDb != nil {
				fmt.Printf("🚨 [DATABASE] Erro ao salvar ticket no histórico: %v\n", errDb)
			}
		}

		fmt.Printf("✅ [WEBHOOK GLPI] Alerta do chamado #%s enviado com sucesso para %s\n", finalID, numeroTI)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}
}

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
