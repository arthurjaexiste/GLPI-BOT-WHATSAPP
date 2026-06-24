package whatsapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bot-glpi/internal/config"
	"bot-glpi/internal/state"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// ─── Helpers ─────────────────────────────────────────────────────────────────

// obterAtendentesSuporte retorna a lista de atendentes configurados no painel.
func obterAtendentesSuporte() []string {
	agentsStr := config.GetConfig().SupportAgents
	var agents []string

	for _, p := range strings.Split(agentsStr, ",") {
		if p = strings.TrimSpace(p); p != "" {
			agents = append(agents, p)
		}
	}

	return agents
}

// ─── Operações de chat ao vivo ────────────────────────────────────────────────

// iniciarChatAoVivo coloca o usuário em atendimento direto ou na fila de espera.
func iniciarChatAoVivo(ctx context.Context, client *whatsmeow.Client, chatJID types.JID, sender string) {
	state.Mu.Lock()

	if state.ActiveLiveChatUser == "" {
		// Atendimento disponível: inicia direto
		state.ActiveLiveChatUser = chatJID.String()
		state.ActiveAgentName = ""
		if uState, ok := state.Users[sender]; ok {
			uState.Step = 100
		}
		nome := state.Names[sender]
		state.Mu.Unlock()

		fmt.Printf("👥 [LIVECHAT] Novo atendimento ao vivo iniciado para %s (%s)\n", nome, sender)
		sendTextMessage(ctx, client, chatJID, formatarMensagem(config.GetConfig().MsgFilaSuporte, nil))
		notificarSuporteNovoAtendimento(ctx, client, nome)
	} else {
		// Atendimento ocupado: coloca na fila
		state.LiveChatQueue = append(state.LiveChatQueue, chatJID.String())
		if uState, ok := state.Users[sender]; ok {
			uState.Step = 99
		}
		pos := len(state.LiveChatQueue)
		nome := state.Names[sender]
		state.Mu.Unlock()

		fmt.Printf("👥 [LIVECHAT] Atendimento ocupado. Adicionando %s (%s) à fila (Posição: %d)\n", nome, sender, pos)
		sendTextMessage(ctx, client, chatJID, formatarMensagem(
			config.GetConfig().MsgFilaEspera,
			map[string]string{"posicao": fmt.Sprintf("%d", pos)},
		))
	}
}

// encerrarChatAoVivo finaliza o atendimento ativo e promove o próximo da fila.
func encerrarChatAoVivo(ctx context.Context, client *whatsmeow.Client, encerradoPeloSuporte bool) {
	state.Mu.Lock()
	currentUserFull := state.ActiveLiveChatUser
	state.ActiveLiveChatUser = ""
	state.ActiveAgentName = ""
	state.Mu.Unlock()

	supportJID := types.NewJID(getSupportNumber(), types.DefaultUserServer)

	if currentUserFull != "" {
		finalizarAtendimentoAtual(ctx, client, currentUserFull, supportJID, encerradoPeloSuporte)
	}

	promoverProximoDaFila(ctx, client, supportJID)
}

// finalizarAtendimentoAtual notifica o usuário e o suporte sobre o encerramento.
func finalizarAtendimentoAtual(ctx context.Context, client *whatsmeow.Client, currentUserFull string, supportJID types.JID, encerradoPeloSuporte bool) {
	userJID, _ := types.ParseJID(currentUserFull)
	userNumber := userJID.User

	state.Mu.Lock()
	nome := state.Names[userNumber]
	if uState, ok := state.Users[userNumber]; ok {
		uState.Step = -1
		uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
	}
	state.Mu.Unlock()

	fmt.Printf("👥 [LIVECHAT] Chat ao vivo encerrado para %s (%s) | Por suporte: %t\n", nome, userNumber, encerradoPeloSuporte)
	sendTextMessage(ctx, client, userJID, formatarMensagem(config.GetConfig().MsgFimAtendimento, nil))

	if !encerradoPeloSuporte {
		sendTextMessage(ctx, client, supportJID, fmt.Sprintf("✅ O usuário *%s* encerrou o chat ao vivo.", nome))
	}
}

// promoverProximoDaFila puxa o próximo usuário da fila para atendimento direto.
func promoverProximoDaFila(ctx context.Context, client *whatsmeow.Client, supportJID types.JID) {
	state.Mu.Lock()

	if len(state.LiveChatQueue) == 0 {
		state.Mu.Unlock()
		sendTextMessage(ctx, client, supportJID, "✅ Chat encerrado com sucesso. A fila de espera está vazia.")
		return
	}

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

	fmt.Printf("👥 [LIVECHAT] Próximo da fila promovido para atendimento: %s (%s)\n", nomeProximo, nextUserNumber)

	sendTextMessage(ctx, client, nextUserJID, "⏳ Chegou a sua vez! Aguarde um momento enquanto um técnico assume o seu atendimento.")
	sendTextMessage(ctx, client, supportJID, fmt.Sprintf("🔔 *NOTIFICAÇÃO FILA:* *%s* saiu da fila de espera e aguarda atendimento.", nomeProximo))
	notificarSuporteNovoAtendimento(ctx, client, nomeProximo)
}

// removerDaFila remove um usuário da fila de espera pelo seu número.
func removerDaFila(sender string) {
	state.Mu.Lock()
	defer state.Mu.Unlock()

	for i, uFull := range state.LiveChatQueue {
		if uJID, _ := types.ParseJID(uFull); uJID.User == sender {
			state.LiveChatQueue = append(state.LiveChatQueue[:i], state.LiveChatQueue[i+1:]...)
			break
		}
	}
}

// notificarSuporteNovoAtendimento envia a enquete de atribuição ao número de suporte.
func notificarSuporteNovoAtendimento(ctx context.Context, client *whatsmeow.Client, nome string) {
	supportJID := types.NewJID(getSupportNumber(), types.DefaultUserServer)
	atendentes := obterAtendentesSuporte()

	fmt.Printf("ℹ️ [LIVECHAT] Enviando enquete de suporte para %s. Atendentes: %v\n", supportJID, atendentes)

	texto := fmt.Sprintf("Quem vai assumir o atendimento de *%s*?", nome)
	pollMsg := client.BuildPollCreation(texto, atendentes, 1)

	if _, err := sendMessage(ctx, client, supportJID, pollMsg); err != nil {
		fmt.Printf("🚨 [ERRO WHATSMEOW] Falha ao enviar enquete de suporte: %v\n", err)
	}
}

// ─── Relay de mensagens do suporte para o usuário ────────────────────────────

// processarMensagemDoSuporte repassa uma mensagem do técnico para o cliente ativo.
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
		_, _ = sendMessage(ctx, client, userJID, v.Message)
	}
}