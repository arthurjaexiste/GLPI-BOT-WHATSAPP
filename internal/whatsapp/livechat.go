package whatsapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"bot-glpi/internal/config"
	"bot-glpi/internal/state"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func obterAtendentesSuporte() []string {
	agentsStr := config.GetConfig().SupportAgents
	var agents []string
	if agentsStr != "" {
		parts := strings.Split(agentsStr, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				agents = append(agents, p)
			}
		}
	}
	if len(agents) == 0 {
		agents = []string{}
	}
	return agents
}

func iniciarChatAoVivo(ctx context.Context, client *whatsmeow.Client, chatJID types.JID, sender string) {
	state.Mu.Lock()
	if state.ActiveLiveChatUser == "" {
		state.ActiveLiveChatUser = chatJID.String()
		state.ActiveAgentName = "" // Começa sem ninguém assinando
		if uState, ok := state.Users[sender]; ok {
			uState.Step = 100
		}
		nome := state.Names[sender]
		state.Mu.Unlock()

		fmt.Printf("👥 [LIVECHAT] Novo atendimento ao vivo iniciado para %s (%s)\n", nome, sender)
		sendTextMessage(ctx, client, chatJID, formatarMensagem(config.GetConfig().MsgFilaSuporte, nil))

		supportJID := types.NewJID(getSupportNumber(), types.DefaultUserServer)

		sendTextMessage(ctx, client, supportJID, fmt.Sprintf("🔔 *NOTIFICAÇÃO:* Novo pedido de chat de *%s*.", nome))
		time.Sleep(1500 * time.Millisecond)

		atendentes := obterAtendentesSuporte()
		fmt.Printf("ℹ️ [LIVECHAT] Enviando enquete de suporte para %s. Atendentes: %v\n", supportJID, atendentes)

		textoNotificacao := fmt.Sprintf("Quem vai assumir o atendimento de *%s*?", nome)
		pollMsg := client.BuildPollCreation(textoNotificacao, atendentes, 1)
		_, errSend := client.SendMessage(ctx, supportJID, pollMsg)
		if errSend != nil {
			fmt.Printf("🚨 [ERRO WHATSMEOW] Falha ao enviar enquete de suporte: %v\n", errSend)
		}
	} else {
		state.LiveChatQueue = append(state.LiveChatQueue, chatJID.String())
		if uState, ok := state.Users[sender]; ok {
			uState.Step = 99
		}
		pos := len(state.LiveChatQueue)
		nome := state.Names[sender]
		state.Mu.Unlock()

		fmt.Printf("👥 [LIVECHAT] Atendimento ocupado. Adicionando %s (%s) à fila de espera (Posição: %d)\n", nome, sender, pos)
		sendTextMessage(ctx, client, chatJID, formatarMensagem(config.GetConfig().MsgFilaEspera, map[string]string{"posicao": strconv.Itoa(pos)}))
	}
}

func encerrarChatAoVivo(ctx context.Context, client *whatsmeow.Client, encerradoPeloSuporte bool) {
	state.Mu.Lock()
	currentUserFull := state.ActiveLiveChatUser
	state.ActiveLiveChatUser = ""
	state.ActiveAgentName = ""
	state.Mu.Unlock()

	supportJID := types.NewJID(getSupportNumber(), types.DefaultUserServer)

	if currentUserFull != "" {
		userJID, _ := types.ParseJID(currentUserFull)
		userNumber := userJID.User

		state.Mu.Lock()
		nome := state.Names[userNumber]
		if uState, ok := state.Users[userNumber]; ok {
			uState.Step = -1
			uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
		}
		state.Mu.Unlock()

		fmt.Printf("👥 [LIVECHAT] Chat ao vivo encerrado para %s (%s) | Encerrado por suporte: %t\n", nome, userNumber, encerradoPeloSuporte)
		sendTextMessage(ctx, client, userJID, formatarMensagem(config.GetConfig().MsgFimAtendimento, nil))

		if !encerradoPeloSuporte {
			sendTextMessage(ctx, client, supportJID, fmt.Sprintf("✅ O usuário *%s* encerrou o chat ao vivo.", nome))
		}
	}

	state.Mu.Lock()
	if len(state.LiveChatQueue) > 0 {
		nextUserFull := state.LiveChatQueue[0]
		state.LiveChatQueue = state.LiveChatQueue[1:]
		state.ActiveLiveChatUser = nextUserFull
		state.ActiveAgentName = ""

		nextUserJID, _ := types.ParseJID(nextUserFull)
		nextUserNumber := nextUserJID.User

		if uState, ok := state.Users[nextUserNumber]; ok {
			uState.Step = 100
		}
		nomeProximo := state.Names[nextUserNumber]
		state.Mu.Unlock()

		fmt.Printf("👥 [LIVECHAT] Próximo da fila de espera puxado para atendimento: %s (%s)\n", nomeProximo, nextUserNumber)

		sendTextMessage(ctx, client, nextUserJID, "⏳ Chegou a sua vez! Aguarde um momento enquanto um técnico assume o seu atendimento.")

		sendTextMessage(ctx, client, supportJID, fmt.Sprintf("🔔 *NOTIFICAÇÃO FILA:* *%s* saiu da fila de espera e aguarda atendimento.", nomeProximo))
		time.Sleep(1500 * time.Millisecond)

		atendentes := obterAtendentesSuporte()
		fmt.Printf("ℹ️ [LIVECHAT] Enviando enquete de suporte (fila) para %s. Atendentes: %v\n", supportJID, atendentes)

		textoNotificacao := fmt.Sprintf("Quem vai assumir o atendimento de *%s*?", nomeProximo)
		pollMsg := client.BuildPollCreation(textoNotificacao, atendentes, 1)
		_, errSend := client.SendMessage(ctx, supportJID, pollMsg)
		if errSend != nil {
			fmt.Printf("🚨 [ERRO WHATSMEOW] Falha ao enviar enquete de suporte (fila): %v\n", errSend)
		}
	} else {
		state.Mu.Unlock()
		sendTextMessage(ctx, client, supportJID, "✅ Chat encerrado com sucesso. A fila de espera está vazia.")
	}
}

func removerDaFila(sender string) {
	state.Mu.Lock()
	defer state.Mu.Unlock()
	for i, uFull := range state.LiveChatQueue {
		uJID, _ := types.ParseJID(uFull)
		if uJID.User == sender {
			state.LiveChatQueue = append(state.LiveChatQueue[:i], state.LiveChatQueue[i+1:]...)
			break
		}
	}
}

func processarMensagemDoSuporte(ctx context.Context, client *whatsmeow.Client, v *events.Message, textoLimpo string, encerrar bool) {
	state.Mu.Lock()
	activeUserFull := state.ActiveLiveChatUser
	agenteAtual := state.ActiveAgentName
	state.Mu.Unlock()

	if activeUserFull == "" {
		return
	}

	if encerrar {
		encerrarChatAoVivo(ctx, client, true)
		return
	}

	userJID, err := types.ParseJID(activeUserFull)
	if err != nil {
		return
	}

	imgMsg := v.Message.GetImageMessage()
	docMsg := v.Message.GetDocumentMessage()
	audioMsg := v.Message.GetAudioMessage()
	videoMsg := v.Message.GetVideoMessage()

	if imgMsg == nil && docMsg == nil && audioMsg == nil && videoMsg == nil {
		sendTextMessage(ctx, client, userJID, fmt.Sprintf("👨‍💻 *%s:*\n\n%s", agenteAtual, textoLimpo))
	} else {
		client.SendMessage(ctx, userJID, v.Message)
	}
}