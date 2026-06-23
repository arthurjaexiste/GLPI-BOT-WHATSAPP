package whatsapp

import (
	"context"
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
	client.SendMessage(ctx, jid, &waE2E.Message{Conversation: proto.String(text)})
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