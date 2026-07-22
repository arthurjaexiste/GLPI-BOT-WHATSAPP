// ============================================================================
// ARQUIVO: livechat.go
// Descrição: Implementação Go (backend) para o ecossistema GLPI-BOT.
// ============================================================================

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

// Função obterAtendentesSuporte executa a regra de negócio/rotina correspondente
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

// Função iniciarChatAoVivo executa a regra de negócio/rotina correspondente
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

// Função encerrarChatAoVivo executa a regra de negócio/rotina correspondente
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

// Função finalizarAtendimentoAtual executa a regra de negócio/rotina correspondente
func finalizarAtendimentoAtual(ctx context.Context, client *whatsmeow.Client, currentUserFull string, supportJID types.JID, encerradoPeloSuporte bool) {
	userJID, _ := types.ParseJID(currentUserFull)
	userNumber := NormalizePhoneLocal(userJID.User)

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

// Função promoverProximoDaFila executa a regra de negócio/rotina correspondente
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
	nextUserNumber := NormalizePhoneLocal(nextUserJID.User)

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

// Função removerDaFila executa a regra de negócio/rotina correspondente
func removerDaFila(sender string) {
	state.Mu.Lock()
	defer state.Mu.Unlock()

	for i, uFull := range state.LiveChatQueue {
		if uJID, _ := types.ParseJID(uFull); NormalizePhoneLocal(uJID.User) == NormalizePhoneLocal(sender) {
			state.LiveChatQueue = append(state.LiveChatQueue[:i], state.LiveChatQueue[i+1:]...)
			break
		}
	}
}

// notificarSuporteNovoAtendimento envia a enquete de atribuição ao número de suporte.
// Se o envio falhar com erro 463 (contato frio / sem conversa recente), faz fallback
// para uma mensagem de texto listando os atendentes disponíveis.

// Função notificarSuporteNovoAtendimento executa a regra de negócio/rotina correspondente
func notificarSuporteNovoAtendimento(ctx context.Context, client *whatsmeow.Client, nome string) {
	supportJID := types.NewJID(getSupportNumber(), types.DefaultUserServer)
	atendentes := obterAtendentesSuporte()

	if len(atendentes) == 0 {
		fmt.Printf("⚠️ [LIVECHAT] Nenhum atendente configurado. Enviando aviso de texto.\n")
		sendTextMessage(ctx, client, supportJID, fmt.Sprintf(
			"🔔 *NOTIFICAÇÃO:* Novo pedido de chat de *%s*.\n\n"+
				"⚠️ Nenhum atendente configurado. Configure a lista em Suporte > Atendentes no painel.",
			nome,
		))
		return
	}

	fmt.Printf("ℹ️ [LIVECHAT] Enviando enquete de suporte para %s. Atendentes: %v\n", supportJID, atendentes)

	texto := fmt.Sprintf("Quem vai assumir o atendimento de *%s*?", nome)
	pollMsg := client.BuildPollCreation(texto, atendentes, 1)

	_, err := sendMessage(ctx, client, supportJID, pollMsg)
	if err == nil {
		return
	}

	fmt.Printf("🚨 [ERRO WHATSMEOW] Falha ao enviar enquete de suporte: %v\n", err)

	// Fallback para erro 463: o número de suporte está como 'contato frio' pois
	// não interagiu com o bot recentemente. Envia texto para que o suporte
	// responda manualmente com o nome do atendente.
	if strings.Contains(err.Error(), "463") {
		fmt.Printf("⚠️ [LIVECHAT] Erro 463 (contato frio). Usando fallback de texto para o suporte.\n")

		lista := ""
		for i, a := range atendentes {
			lista += fmt.Sprintf("  *%d.* %s\n", i+1, a)
		}

		sendTextMessage(ctx, client, supportJID, fmt.Sprintf(
			"🔔 *NOTIFICAÇÃO:* Novo pedido de chat de *%s*.\n\n"+
				"📋 *Atendentes disponíveis:*\n%s\n"+
				"↩️ Responda com o seu nome *exatamente* como listado acima para assumir o atendimento.\n\n"+
				"💡 _Para restaurar o menu de seleção, envie qualquer mensagem para o bot._",
			nome, lista,
		))
	}
}

// ─── Relay de mensagens do suporte para o usuário ────────────────────────────

// processarMensagemDoSuporte repassa uma mensagem do técnico para o cliente ativo.

// Função processarMensagemDoSuporte executa a regra de negócio/rotina correspondente
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