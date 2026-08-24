// ============================================================================
// ARQUIVO: utils.go
// Descrição: Implementação Go (backend) para o ecossistema GLPI-BOT.
// ============================================================================

package whatsapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"bot-glpi/internal/config"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

// ─── Helpers de configuração ─────────────────────────────────────────────────

// getEmpresa retorna o nome da empresa configurado, com fallback "TI".

// Função getEmpresa executa a regra de negócio/rotina correspondente
func getEmpresa() string {
	if nome := config.GetConfig().CompanyName; nome != "" {
		return nome
	}
	return "TI"
}

// getSaudacao retorna a saudação correta conforme o horário de Brasília.

// Função getSaudacao executa a regra de negócio/rotina correspondente
func getSaudacao() string {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	agora := time.Now()
	if err == nil {
		agora = agora.In(loc)
	}

	switch h := agora.Hour(); {
	case h >= 5 && h < 12:
		return "Bom dia"
	case h >= 12 && h < 18:
		return "Boa tarde"
	default:
		return "Boa noite"
	}
}

// getSupportNumber retorna o número de suporte configurado, sem formatação.

// Função getSupportNumber executa a regra de negócio/rotina correspondente
func getSupportNumber() string {
	num := config.GetConfig().TelefoneNotificacao
	num = strings.NewReplacer("+", "", "-", "", " ", "").Replace(num)
	return strings.TrimSpace(num)
}

// isBlacklisted verifica se um número está na lista de bloqueados (DarkList).

// Função isBlacklisted executa a regra de negócio/rotina correspondente
func isBlacklisted(sender string) bool {
	darkListStr := config.GetConfig().DarkList
	if darkListStr == "" {
		return false
	}

	senderClean := strings.Split(sender, ":")[0]

	for _, num := range strings.Split(darkListStr, ",") {
		num = strings.NewReplacer("+", "", "-", "").Replace(strings.TrimSpace(num))
		if num == "" {
			continue
		}
		if phonesSufixMatch(senderClean, num, 8) {
			return true
		}
	}

	return false
}

// ─── Envio de mensagens ───────────────────────────────────────────────────────

var (
	normalizedJIDsMu sync.RWMutex
	normalizedJIDs   = make(map[string]types.JID)
)

// normalizarJID verifica se o JID é brasileiro e resolve o formato correto (8 ou 9 dígitos)
// consultando o servidor do WhatsApp caso não esteja em cache.

// Função normalizarJID executa a regra de negócio/rotina correspondente
func normalizarJID(ctx context.Context, client *whatsmeow.Client, jid types.JID) types.JID {
	if client == nil || jid.Server != types.DefaultUserServer {
		return jid
	}

	// Verifica se é um número brasileiro (DDI 55)
	if !strings.HasPrefix(jid.User, "55") {
		return jid
	}

	normalizedJIDsMu.RLock()
	cached, exists := normalizedJIDs[jid.User]
	normalizedJIDsMu.RUnlock()
	if exists {
		return cached
	}

	var queryNumbers []string
	user := jid.User

	// Determina variantes (com e sem o 9º dígito)
	if len(user) == 13 && user[4] == '9' {
		// Formato 9 dígitos: e.g. 55 11 9 1234 5678
		// Variante 8 dígitos: e.g. 55 11 1234 5678
		eightDigit := user[:4] + user[5:]
		queryNumbers = []string{"+" + user, "+" + eightDigit}
	} else if len(user) == 12 {
		// Formato 8 dígitos: e.g. 55 11 1234 5678
		// Variante 9 dígitos: e.g. 55 11 9 1234 5678
		nineDigit := user[:4] + "9" + user[4:]
		queryNumbers = []string{"+" + user, "+" + nineDigit}
	} else {
		queryNumbers = []string{"+" + user}
	}

	// Consulta o WhatsApp para validar qual versão é a registrada
	resp, err := client.IsOnWhatsApp(ctx, queryNumbers)
	if err == nil {
		for _, r := range resp {
			if r.IsIn {
				normalizedJIDsMu.Lock()
				normalizedJIDs[user] = r.JID
				normalizedJIDs[r.JID.User] = r.JID
				normalizedJIDsMu.Unlock()
				fmt.Printf("ℹ️ [JID NORMALIZER] JID original: %s | JID resolvido/correto: %s\n", jid.String(), r.JID.String())
				return r.JID
			}
		}
	}

	// Caso falhe ou não encontre, retorna o JID original como fallback
	return jid
}

// sendTextMessage envia uma mensagem de texto simples para um JID.

// Função sendTextMessage executa a regra de negócio/rotina correspondente
func sendTextMessage(ctx context.Context, client *whatsmeow.Client, jid types.JID, text string) {
	_, _ = sendMessage(ctx, client, jid, &waE2E.Message{Conversation: proto.String(text)})
}

// sendQuotedTextMessage envia uma mensagem citando nativamente outra mensagem no WhatsApp.
func sendQuotedTextMessage(ctx context.Context, client *whatsmeow.Client, jid types.JID, text string, quotedJID string, quotedText string, quotedMsgID string) {
	participant := quotedJID
	if participant == "" {
		participant = jid.String()
	}
	if pJID, err := types.ParseJID(participant); err == nil {
		participant = pJID.ToNonAD().String()
	}
	if strings.HasSuffix(participant, "@c.us") {
		participant = strings.TrimSuffix(participant, "@c.us") + "@s.whatsapp.net"
	}

	contextInfo := &waE2E.ContextInfo{
		Participant: proto.String(participant),
		QuotedMessage: &waE2E.Message{
			Conversation: proto.String(quotedText),
		},
	}

	if quotedMsgID != "" {
		contextInfo.StanzaID = proto.String(quotedMsgID)
	}

	msg := &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        proto.String(text),
			ContextInfo: contextInfo,
		},
	}

	_, _ = sendMessage(ctx, client, jid, msg)
}

// sendMessage envia qualquer tipo de mensagem, simulando digitação para chats de usuário.

// Função sendMessage executa a regra de negócio/rotina correspondente
func sendMessage(ctx context.Context, client *whatsmeow.Client, jid types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
	if client == nil {
		return whatsmeow.SendResponse{}, fmt.Errorf("cliente whatsmeow nulo")
	}

	// Normaliza o JID antes de qualquer operação (incluindo simulação de digitação)
	jid = normalizarJID(ctx, client, jid)

	// Não simula digitação para o chat interno do suporte/TI
	if !strings.Contains(jid.String(), getSupportNumber()) {
		simularDigitacao(ctx, client, jid, msg)
	}

	resp, err := client.SendMessage(ctx, jid, msg)
	if err == nil {
		if webDB != nil {
			msgText := ""
			msgType := "text"
			if msg.Conversation != nil {
				msgText = *msg.Conversation
			} else if msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.Text != nil {
				msgText = *msg.ExtendedTextMessage.Text
			} else if msg.AudioMessage != nil {
				msgText = "[Áudio]"
				msgType = "audio"
			} else if msg.ImageMessage != nil {
				msgText = "[Imagem]"
				msgType = "image"
			} else if msg.DocumentMessage != nil {
				msgText = "[Documento]"
				msgType = "document"
			} else if msg.PollCreationMessage != nil {
				msgText = "[Enquete]"
				msgType = "poll"
			}

			if msgText != "" {
				_, _ = webDB.Exec(
					"INSERT INTO chat_messages (chat_jid, sender_name, sender_jid, message_text, message_type, is_from_me, wa_message_id) VALUES (?, ?, ?, ?, ?, 1, ?)",
					jid.String(), "GLPI-BOT (Bot)", "", msgText, msgType, resp.ID,
				)
			}
		}
	}
	if err != nil {
		fmt.Printf("🚨 [ERRO WHATSAPP] Falha ao enviar mensagem para %s: %v\n", jid.String(), err)
		if strings.Contains(err.Error(), "463") {
			fmt.Printf(
				"💡 [DICA] O número %s pode estar bloqueado como 'contato frio'. "+
					"Envie 'oi' deste celular para o bot para liberar.\n",
				jid.String(),
			)
		}
	}

	return resp, err
}

// simularDigitacao envia o evento de "digitando..." e aguarda um tempo proporcional
// ao tamanho da mensagem antes de enviá-la, tornando a experiência mais natural.

// Função simularDigitacao executa a regra de negócio/rotina correspondente
func simularDigitacao(ctx context.Context, client *whatsmeow.Client, jid types.JID, msg *waE2E.Message) {
	mediaType := types.ChatPresenceMediaText
	if msg.AudioMessage != nil {
		mediaType = types.ChatPresenceMediaAudio
	}

	_ = client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, mediaType)

	delay := calcularDelay(msg)
	time.Sleep(delay)

	_ = client.SendChatPresence(ctx, jid, types.ChatPresencePaused, mediaType)
}

// calcularDelay estima o tempo de "digitação" com base no tamanho do texto.

// Função calcularDelay executa a regra de negócio/rotina correspondente
func calcularDelay(msg *waE2E.Message) time.Duration {
	const (
		minDelay     = 150 * time.Millisecond
		maxDelay     = 350 * time.Millisecond
		defaultDelay = 150 * time.Millisecond
		audioDelay   = 500 * time.Millisecond
		msPerChar    = 2 * time.Millisecond
	)

	var textLength int
	switch {
	case msg.Conversation != nil:
		textLength = len(*msg.Conversation)
	case msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.Text != nil:
		textLength = len(*msg.ExtendedTextMessage.Text)
	case msg.AudioMessage != nil:
		return audioDelay
	}

	if textLength == 0 {
		return defaultDelay
	}

	delay := time.Duration(textLength) * msPerChar
	if delay < minDelay {
		return minDelay
	}
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}

// ─── Formatação de mensagens ──────────────────────────────────────────────────

// formatarMensagem substitui placeholders como {empresa}, {saudacao} e chaves customizadas.

// Função formatarMensagem executa a regra de negócio/rotina correspondente
func formatarMensagem(msg string, placeholders map[string]string) string {
	res := msg
	for k, v := range placeholders {
		res = strings.ReplaceAll(res, "{"+k+"}", v)
	}
	res = strings.ReplaceAll(res, "{empresa}", getEmpresa())
	res = strings.ReplaceAll(res, "{saudacao}", getSaudacao())
	return res
}

// unwrapMessage desembrulha mensagens do WhatsApp envoltas em contêineres como Ephemeral, ViewOnce, etc.

// Função unwrapMessage executa a regra de negócio/rotina correspondente
func unwrapMessage(msg *waE2E.Message) *waE2E.Message {
	if msg == nil {
		return nil
	}
	if msg.GetEphemeralMessage() != nil && msg.GetEphemeralMessage().GetMessage() != nil {
		return unwrapMessage(msg.GetEphemeralMessage().GetMessage())
	}
	if msg.GetViewOnceMessage() != nil && msg.GetViewOnceMessage().GetMessage() != nil {
		return unwrapMessage(msg.GetViewOnceMessage().GetMessage())
	}
	if msg.GetViewOnceMessageV2() != nil && msg.GetViewOnceMessageV2().GetMessage() != nil {
		return unwrapMessage(msg.GetViewOnceMessageV2().GetMessage())
	}
	if msg.GetViewOnceMessageV2Extension() != nil && msg.GetViewOnceMessageV2Extension().GetMessage() != nil {
		return unwrapMessage(msg.GetViewOnceMessageV2Extension().GetMessage())
	}
	if msg.GetDocumentWithCaptionMessage() != nil && msg.GetDocumentWithCaptionMessage().GetMessage() != nil {
		return unwrapMessage(msg.GetDocumentWithCaptionMessage().GetMessage())
	}
	return msg
}

// extrairConteudoMensagem extrai o texto e possíveis mídias de um evento de mensagem.

// Função extrairConteudoMensagem executa a regra de negócio/rotina correspondente
func extrairConteudoMensagem(v *events.Message) (string, *waE2E.ImageMessage, *waE2E.DocumentMessage, *waE2E.VideoMessage, *waE2E.AudioMessage) {
	msg := unwrapMessage(v.Message)
	if msg == nil {
		return "", nil, nil, nil, nil
	}

	imgMsg := msg.GetImageMessage()
	docMsg := msg.GetDocumentMessage()
	videoMsg := msg.GetVideoMessage()
	audioMsg := msg.GetAudioMessage()

	var rawText string
	switch {
	case msg.GetExtendedTextMessage() != nil:
		rawText = msg.GetExtendedTextMessage().GetText()
	case imgMsg != nil:
		rawText = imgMsg.GetCaption()
	case docMsg != nil:
		rawText = docMsg.GetCaption()
	case videoMsg != nil:
		rawText = videoMsg.GetCaption()
	case msg.GetPollUpdateMessage() == nil:
		rawText = msg.GetConversation()
	}

	return rawText, imgMsg, docMsg, videoMsg, audioMsg
}

// ─── Horário de atendimento ───────────────────────────────────────────────────

// IsOutsideWorkingHours retorna true se o momento atual está fora do horário configurado.

// Função IsOutsideWorkingHours executa a regra de negócio/rotina correspondente
func IsOutsideWorkingHours() bool {
	cfg := config.GetConfig()
	if !cfg.WorkingHoursEnabled {
		return false
	}

	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.Local
	}
	now := time.Now().In(loc)

	if !isDiaUtil(now, cfg.WorkingDays) {
		return true
	}

	return !estaNoPeriodo(now, cfg.WorkingHoursStart, cfg.WorkingHoursEnd)
}

// isDiaUtil verifica se o dia da semana atual está na lista de dias de trabalho.

// Função isDiaUtil executa a regra de negócio/rotina correspondente
func isDiaUtil(t time.Time, workingDays string) bool {
	weekday := int(t.Weekday())
	for _, dayStr := range strings.Split(workingDays, ",") {
		if d, err := strconv.Atoi(strings.TrimSpace(dayStr)); err == nil && d == weekday {
			return true
		}
	}
	return false
}

// estaNoPeriodo verifica se o horário atual está entre start e end (no formato "HH:MM").

// Função estaNoPeriodo executa a regra de negócio/rotina correspondente
func estaNoPeriodo(t time.Time, start, end string) bool {
	parsarMinutos := func(s string) (int, bool) {
		parts := strings.Split(s, ":")
		if len(parts) != 2 {
			return 0, false
		}
		h, errH := strconv.Atoi(parts[0])
		m, errM := strconv.Atoi(parts[1])
		if errH != nil || errM != nil {
			return 0, false
		}
		return h*60 + m, true
	}

	startMin, okS := parsarMinutos(start)
	endMin, okE := parsarMinutos(end)
	if !okS || !okE {
		return true // Configuração inválida: considera dentro do horário por segurança
	}

	current := t.Hour()*60 + t.Minute()
	return current >= startMin && current <= endMin
}

// ─── URL do GLPI ──────────────────────────────────────────────────────────────

// obterLinkTicketGLPI monta o link direto para um ticket no painel web do GLPI.

// Função obterLinkTicketGLPI executa a regra de negócio/rotina correspondente
func obterLinkTicketGLPI(ticketID string) string {
	apiURL := config.GetConfig().GLPIApiURL
	baseURL := strings.TrimSuffix(apiURL, "/")
	baseURL = strings.TrimSuffix(strings.ReplaceAll(baseURL, "/apirest.php", ""), "/")
	return fmt.Sprintf("%s/index.php?redirect=ticket_%s", baseURL, ticketID)
}

// NormalizePhoneLocal normaliza o número de telefone localmente,
// removendo formatação, sufixos e padronizando números brasileiros para 8 dígitos.

// Função NormalizePhoneLocal executa a regra de negócio/rotina correspondente
func NormalizePhoneLocal(phone string) string {
	phone = strings.Split(phone, "@")[0]
	phone = strings.Split(phone, ":")[0]
	phone = strings.NewReplacer("+", "", "-", "", " ", "").Replace(phone)

	if strings.HasPrefix(phone, "55") && len(phone) >= 12 {
		ddd := phone[2:4]
		rest := phone[4:]
		if len(rest) == 9 && rest[0] == '9' {
			rest = rest[1:]
		}
		return "55" + ddd + rest
	}
	if !strings.HasPrefix(phone, "55") && (len(phone) == 10 || len(phone) == 11) {
		ddd := phone[0:2]
		rest := phone[2:]
		if len(rest) == 9 && rest[0] == '9' {
			rest = rest[1:]
		}
		return "55" + ddd + rest
	}
	return phone
}
