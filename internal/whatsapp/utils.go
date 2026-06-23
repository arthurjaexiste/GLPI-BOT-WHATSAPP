package whatsapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bot-glpi/internal/config"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func getEmpresa() string {
	nome := config.GetConfig().CompanyName
	if nome == "" {
		return "TI"
	}
	return nome
}

func getSaudacao() string {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	var agora time.Time
	if err != nil {
		agora = time.Now()
	} else {
		agora = time.Now().In(loc)
	}
	hora := agora.Hour()
	if hora >= 5 && hora < 12 {
		return "Bom dia"
	} else if hora >= 12 && hora < 18 {
		return "Boa tarde"
	}
	return "Boa noite"
}

// Limpa formatações do número de suporte do .env
func getSupportNumber() string {
	num := config.GetConfig().TelefoneNotificacao
	num = strings.ReplaceAll(num, "+", "")
	num = strings.ReplaceAll(num, "-", "")
	num = strings.ReplaceAll(num, " ", "")
	return strings.TrimSpace(num)
}

func isBlacklisted(sender string) bool {
	darkListStr := config.GetConfig().DarkList
	if darkListStr == "" {
		return false
	}

	senderClean := strings.Split(sender, ":")[0]
	darkList := strings.Split(darkListStr, ",")

	for _, num := range darkList {
		num = strings.TrimSpace(num)
		num = strings.ReplaceAll(num, "+", "")
		num = strings.ReplaceAll(num, "-", "")

		if num == "" {
			continue
		}

		if len(senderClean) >= 8 && len(num) >= 8 {
			if senderClean[len(senderClean)-8:] == num[len(num)-8:] {
				return true
			}
		} else if senderClean == num {
			return true
		}
	}
	return false
}

func sendTextMessage(ctx context.Context, client *whatsmeow.Client, jid types.JID, text string) {
	_, _ = sendMessage(ctx, client, jid, &waE2E.Message{Conversation: proto.String(text)})
}

func formatarMensagem(msg string, placeholders map[string]string) string {
	res := msg
	for k, v := range placeholders {
		res = strings.ReplaceAll(res, "{"+k+"}", v)
	}
	res = strings.ReplaceAll(res, "{empresa}", getEmpresa())
	res = strings.ReplaceAll(res, "{saudacao}", getSaudacao())
	return res
}

func extrairConteudoMensagem(v *events.Message) (string, *waE2E.ImageMessage, *waE2E.DocumentMessage) {
	var rawText string
	imgMsg := v.Message.GetImageMessage()
	docMsg := v.Message.GetDocumentMessage()

	if v.Message.GetExtendedTextMessage() != nil {
		rawText = v.Message.GetExtendedTextMessage().GetText()
	} else if imgMsg != nil {
		rawText = imgMsg.GetCaption()
	} else if docMsg != nil {
		rawText = docMsg.GetCaption()
	} else if v.Message.GetPollUpdateMessage() == nil {
		rawText = v.Message.GetConversation()
	}
	return rawText, imgMsg, docMsg
}

func sendMessage(ctx context.Context, client *whatsmeow.Client, jid types.JID, msg *waE2E.Message) (whatsmeow.SendResponse, error) {
	if client == nil {
		return whatsmeow.SendResponse{}, fmt.Errorf("cliente whatsmeow nulo")
	}

	// Não simula digitação para alertas internos (ex: webhook enviado para o suporte/TI)
	tiNum := getSupportNumber()
	isTiChat := strings.Contains(jid.String(), tiNum)

	if !isTiChat {
		presenceState := types.ChatPresenceComposing
		mediaType := types.ChatPresenceMediaText

		if msg.AudioMessage != nil {
			mediaType = types.ChatPresenceMediaAudio
		}

		_ = client.SendChatPresence(ctx, jid, presenceState, mediaType)

		// Calcula tempo baseado na mensagem (velocidade média de escrita)
		delay := 1200 * time.Millisecond
		var textLength int
		if msg.Conversation != nil {
			textLength = len(*msg.Conversation)
		} else if msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.Text != nil {
			textLength = len(*msg.ExtendedTextMessage.Text)
		}

		if textLength > 0 {
			calcDelay := time.Duration(textLength) * 12 * time.Millisecond
			if calcDelay < 1000*time.Millisecond {
				delay = 1000 * time.Millisecond
			} else if calcDelay > 2500*time.Millisecond {
				delay = 2500 * time.Millisecond
			} else {
				delay = calcDelay
			}
		} else if msg.AudioMessage != nil {
			delay = 3000 * time.Millisecond
		}

		time.Sleep(delay)
		_ = client.SendChatPresence(ctx, jid, types.ChatPresencePaused, mediaType)
	}

	resp, err := client.SendMessage(ctx, jid, msg)
	if err != nil {
		fmt.Printf("🚨 [ERRO WHATSAPP] Falha ao enviar mensagem para %s: %v\n", jid.String(), err)
		if strings.Contains(err.Error(), "463") {
			fmt.Printf("💡 [DICA] O número %s pode estar bloqueado como 'contato frio' pelo WhatsApp após a recriação da sessão. Para liberar, basta enviar qualquer mensagem (ex: 'oi') deste celular para o número do bot.\n", jid.String())
		}
	}
	return resp, err
}

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

	// 1. Verifica se o dia atual da semana é dia útil configurado
	weekday := int(now.Weekday()) // 0=Domingo, 1=Segunda, ..., 6=Sábado
	daysParts := strings.Split(cfg.WorkingDays, ",")
	isWorkingDay := false
	for _, dayStr := range daysParts {
		dayStr = strings.TrimSpace(dayStr)
		if dayStr == "" {
			continue
		}
		d, err := strconv.Atoi(dayStr)
		if err == nil && d == weekday {
			isWorkingDay = true
			break
		}
	}

	if !isWorkingDay {
		return true // Fora do expediente porque não é dia de trabalho
	}

	// 2. Verifica o horário (ex: "08:00" até "18:00")
	startParts := strings.Split(cfg.WorkingHoursStart, ":")
	endParts := strings.Split(cfg.WorkingHoursEnd, ":")
	if len(startParts) != 2 || len(endParts) != 2 {
		return false // Configuração inválida, considera dentro do horário por segurança
	}

	startHour, _ := strconv.Atoi(startParts[0])
	startMin, _ := strconv.Atoi(startParts[1])
	endHour, _ := strconv.Atoi(endParts[0])
	endMin, _ := strconv.Atoi(endParts[1])

	currentMinutes := now.Hour()*60 + now.Minute()
	startMinutes := startHour*60 + startMin
	endMinutes := endHour*60 + endMin

	if currentMinutes < startMinutes || currentMinutes > endMinutes {
		return true // Fora do horário de expediente diário
	}

	return false
}

func obterLinkTicketGLPI(ticketID string) string {
	apiURL := config.GetConfig().GLPIApiURL
	baseWebURL := apiURL
	baseWebURL = strings.TrimSuffix(baseWebURL, "/")
	baseWebURL = strings.Replace(baseWebURL, "/apirest.php", "", 1)
	baseWebURL = strings.Replace(baseWebURL, "apirest.php", "", 1)
	baseWebURL = strings.TrimSuffix(baseWebURL, "/")
	
	return fmt.Sprintf("%s/index.php?redirect=ticket_%s", baseWebURL, ticketID)
}