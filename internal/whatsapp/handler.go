package whatsapp

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"bot-glpi/internal/state"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	_ "modernc.org/sqlite"
)

// ==========================================
// VARIÁVEIS GLOBAIS E ESTADO (COMPARTILHADO)
// ==========================================
var (
	CurrentQR       string
	IsConnected     bool
	ClientMu        sync.Mutex
	GlobalClient    *whatsmeow.Client
	GlobalContainer *sqlstore.Container

	// Variáveis do Sistema de Login Web
	webDB      *sql.DB
	sessions   = make(map[string]time.Time)
	sessionsMu sync.Mutex
)

func resetarEstadoUsuario(sender string, uState *state.UserState) {
	state.Mu.Lock()
	uState.Step = -1
	uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
	uState.Images = nil
	uState.Docs = nil
	uState.SubCategory = ""
	uState.ActiveTicketID = 0
	state.Mu.Unlock()
}

func retrocederPasso(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string) {
	state.Mu.Lock()
	passoAtual := uState.Step

	switch passoAtual {
	case 10, 11, 12, 30:
		uState.Step = -1
		uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, "Atendimento reiniciado. Envie uma mensagem para começar de novo.")

	case 50:
		state.Mu.Unlock()
		SendRootFlowPoll(ctx, client, v.Info.Chat, uState)

	case 51, 52:
		uState.Step = 50
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, "Voltando... Por favor digite o **número do chamado**:")

	case 1000:
		currentNodeID := uState.CurrentNodeID
		parent, found := FindParentNodeByID(currentNodeID)
		if found && parent.ID != "" {
			var options []string
			for _, child := range parent.Children {
				options = append(options, child.Title)
			}
			uState.CurrentNodeID = parent.ID
			state.Mu.Unlock()

			var pollMsg *waE2E.Message
			if parent.ID == "root" {
				pollMsg = client.BuildPollCreation("Como posso te ajudar hoje?", options, 1)
			} else {
				pollMsg = client.BuildPollCreation(fmt.Sprintf("Qual o problema com %s?", parent.Title), options, 1)
			}
			_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
		} else {
			state.Mu.Unlock()
			SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
		}

	case 2:
		uState.Title = ""
		currentNodeID := uState.CurrentNodeID
		parent, found := FindParentNodeByID(currentNodeID)
		if found {
			var options []string
			for _, child := range parent.Children {
				options = append(options, child.Title)
			}
			uState.Step = 1000
			uState.CurrentNodeID = parent.ID
			state.Mu.Unlock()

			var pollMsg *waE2E.Message
			if parent.ID == "root" {
				pollMsg = client.BuildPollCreation("Como posso te ajudar hoje?", options, 1)
			} else {
				pollMsg = client.BuildPollCreation(fmt.Sprintf("Qual o problema com %s?", parent.Title), options, 1)
			}
			_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
		} else {
			state.Mu.Unlock()
			SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
		}

	case 3:
		uState.Step = 2
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, "Voltando... Por favor, digite um **título curto** para o problema:")

	case 35, 36, 37, 40, 41, 42:
		uState.Step = 3
		uState.Images = nil
		uState.Docs = nil
		state.Mu.Unlock()

		prompt := obterPromptDescricaoDinamico(uState)
		sendTextMessage(ctx, client, v.Info.Chat, "Voltando... "+prompt)

	default:
		state.Mu.Unlock()
	}
}

func HandleMessage(client *whatsmeow.Client, evt interface{}) {
	v, ok := evt.(*events.Message)
	if !ok || v.Info.IsFromMe || v.Info.IsGroup {
		return
	}

	ctx := context.Background()
	sender := v.Info.Chat.User
	chatJID := v.Info.Chat
	pollUpdate := v.Message.GetPollUpdateMessage()

	rawText, imgMsg, docMsg := extrairConteudoMensagem(v)
	text := strings.TrimSpace(rawText)
	textLower := strings.ToLower(text)

	client.MarkRead(ctx, []types.MessageID{v.Info.ID}, v.Info.Timestamp, chatJID, v.Info.Sender)

	state.Mu.Lock()
	userName := state.Names[sender]
	userStep := -1
	if uState, exists := state.Users[sender]; exists {
		userStep = uState.Step
	}
	activeUserFull := state.ActiveLiveChatUser
	state.Mu.Unlock()

	var debugName string
	if userName != "" {
		debugName = fmt.Sprintf("%s (%s)", userName, sender)
	} else {
		debugName = sender
	}

	if pollUpdate != nil {
		fmt.Printf("📥 [ENQUETE] Usuário %s votou em uma enquete do WhatsApp.\n", debugName)
	} else if userStep == 100 && activeUserFull != "" {
		if imgMsg != nil {
			fmt.Printf("💬 [LIVECHAT] Cliente %s em atendimento enviou uma imagem\n", debugName)
		} else if docMsg != nil {
			fmt.Printf("💬 [LIVECHAT] Cliente %s em atendimento enviou um documento (%q)\n", debugName, docMsg.GetFileName())
		} else {
			fmt.Printf("💬 [LIVECHAT] Cliente %s em atendimento enviou mensagem: %q\n", debugName, text)
		}
	} else {
		if imgMsg != nil {
			fmt.Printf("📥 [IMAGEM] Usuário %s enviou uma imagem | Passo: %d\n", debugName, userStep)
		} else if docMsg != nil {
			fmt.Printf("📥 [DOCUMENTO] Usuário %s enviou um documento (%q) | Passo: %d\n", debugName, docMsg.GetFileName(), userStep)
		} else {
			fmt.Printf("📥 [MENSAGEM] Usuário %s enviou: %q | Passo: %d\n", debugName, text, userStep)
		}
	}

	supportNumber := getSupportNumber()
	isSupport := false
	if len(sender) >= 8 && len(supportNumber) >= 8 {
		if sender[len(sender)-8:] == supportNumber[len(supportNumber)-8:] {
			isSupport = true
		}
	}

	// 1. A TRAVA DE FERRO DO SUPORTE
	if strings.HasPrefix(text, "!") {
		if activeUserFull != "" {
			textoLimpo := strings.TrimSpace(text[1:])

			state.Mu.Lock()
			agenteAtual := state.ActiveAgentName
			state.Mu.Unlock()

			fmt.Printf("👤 [LIVECHAT] Suporte (%s) enviou resposta para o cliente (%s): %q\n", agenteAtual, activeUserFull, textoLimpo)

			if agenteAtual == "" {
				supportJID := types.NewJID(getSupportNumber(), types.DefaultUserServer)
				sendTextMessage(ctx, client, supportJID, "⚠️ *Atenção:* Você precisa assumir o atendimento na enquete primeiro.")
				return
			}

			processarMensagemDoSuporte(ctx, client, v, textoLimpo, false)
		} else if isSupport {
			sendTextMessage(ctx, client, chatJID, "⚠️ *Aviso do Sistema:* Não há nenhum usuário no chat ao vivo no momento.")
		}
		return
	}

	if textLower == "#encerrar" && activeUserFull != "" {
		encerradoPeloSuporte := true
		activeJID, _ := types.ParseJID(activeUserFull)
		if sender == activeJID.User {
			encerradoPeloSuporte = false
		}
		cerrarChatAoVivo := encerrarChatAoVivo
		cerrarChatAoVivo(ctx, client, encerradoPeloSuporte)
		return
	}

	// BARREIRA DA DARK LIST & PROTEÇÃO DO SUPORTE
	if isSupport || isBlacklisted(sender) {
		return
	}

	// 2. ATENDIMENTO NORMAL DE USUÁRIO
	state.Mu.Lock()
	uState, exists := state.Users[sender]
	if !exists {
		uState = &state.UserState{Step: -1, LastGreetingTime: time.Now().Add(-15 * time.Minute)}
		state.Users[sender] = uState
	}
	currentStep := uState.Step
	state.Mu.Unlock()

	// INTERCEPTA O MODO CHAT DO USUÁRIO ENVIANDO PARA O TÉCNICO
	if currentStep == 100 {
		if textLower == "#encerrar" || textLower == "cancelar" || textLower == "sair" || text == "#" {
			encerrarChatAoVivo(ctx, client, false)
			return
		}

		supportJID := types.NewJID(getSupportNumber(), types.DefaultUserServer)
		state.Mu.Lock()
		nome := state.Names[sender]
		state.Mu.Unlock()

		audioMsg := v.Message.GetAudioMessage()
		videoMsg := v.Message.GetVideoMessage()
		stickerMsg := v.Message.GetStickerMessage()

		if imgMsg == nil && docMsg == nil && audioMsg == nil && videoMsg == nil && stickerMsg == nil {
			// Apenas texto
			sendTextMessage(ctx, client, supportJID, fmt.Sprintf("👤 *%s:*\n\n%s", nome, text))
		} else {
			// É uma mídia. Envia a etiqueta e em seguida a mensagem com o arquivo real!
			sendTextMessage(ctx, client, supportJID, fmt.Sprintf("👤 *%s enviou o arquivo/mídia abaixo:*", nome))
			_, _ = sendMessage(ctx, client, supportJID, v.Message)
		}
		return
	}

	if currentStep == 99 {
		if textLower == "#cancelar" || textLower == "cancelar" || textLower == "sair" || text == "#" {
			removerDaFila(sender)
			state.Mu.Lock()
			uState.Step = -1
			uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
			state.Mu.Unlock()
			sendTextMessage(ctx, client, chatJID, "✅ Você saiu da fila de espera. Quando precisar de algo, é só mandar uma nova mensagem! 🚀")
		} else {
			sendTextMessage(ctx, client, chatJID, "⏳ Você está na fila de espera para falar com o suporte. Se quiser desistir, digite *#cancelar*.")
		}
		return
	}

	if textLower == "cancelar" || textLower == "sair" || text == "#" {
		if currentStep != -1 {
			resetarEstadoUsuario(sender, uState)
			sendTextMessage(ctx, client, chatJID, "✅ Atendimento cancelado. Quando precisar, envie uma nova mensagem para recomeçar! 🚀")
		}
		return
	}
	if text == "*" {
		if currentStep != -1 {
			retrocederPasso(ctx, client, v, uState, sender)
			return
		}
	}

	if pollUpdate != nil {
		HandlePollUpdate(ctx, client, v, uState, sender)
		return
	}

	if currentStep >= 35 && currentStep <= 42 {
		processarMidiasEAnexos(ctx, client, v, uState, sender, textLower, imgMsg, docMsg)
		return
	}

	switch currentStep {
	case -1:
		processarSaudacaoInicial(ctx, client, v, uState, sender)
	case 1:
		processarBuscaNomeGLPI(ctx, client, v, uState, sender, text)
	case 12:
		processarNomeManual(ctx, client, v, uState, sender, text)
	case 2:
		processarGravacaoTitulo(ctx, client, v, uState, text, imgMsg, docMsg)
	case 3:
		processarGravacaoDescricao(ctx, client, v, uState, sender, text, imgMsg, docMsg)
	case 50:
		processarBuscaChamadoInfo(ctx, client, v, uState, sender, text)
	case 52:
		processarNovaMensagemChamado(ctx, client, v, uState, text)
	}
}

func StartWhatsApp(ctx context.Context) error {
	ClientMu.Lock()
	defer ClientMu.Unlock()

	if GlobalClient != nil {
		GlobalClient.Disconnect()
	}

	if GlobalContainer == nil {
		return fmt.Errorf("GlobalContainer não está configurado")
	}

	deviceStore, err := GlobalContainer.GetFirstDevice(ctx)
	if err != nil {
		return fmt.Errorf("erro ao obter device store: %w", err)
	}

	client := whatsmeow.NewClient(deviceStore, nil)
	GlobalClient = client
	client.AddEventHandler(GetEventHandler(client))

	if client.Store.ID == nil {
		// Sem sessão salva: vai pedir QR Code
		qrChan, err := client.GetQRChannel(ctx)
		if err != nil {
			return fmt.Errorf("erro ao obter canal de QR Code: %w", err)
		}
		err = client.Connect()
		if err != nil {
			return fmt.Errorf("erro ao conectar: %w", err)
		}
		go func() {
			for evt := range qrChan {
				if evt.Event == "code" {
					CurrentQR = evt.Code
					IsConnected = false
					fmt.Println("⚠️  NOVO QR CODE GERADO. VEJA NO PAINEL WEB OU ESCANEIE.")
				} else if evt.Event == "success" {
					IsConnected = true
					CurrentQR = ""
					fmt.Println("✅ Bot conectado ao WhatsApp via QR Code!")
				}
			}
		}()
	} else {
		// Já possui sessão salva: conecta direto
		err = client.Connect()
		if err != nil {
			return fmt.Errorf("erro ao conectar: %w", err)
		}
	}

	return nil
}

func GetEventHandler(client *whatsmeow.Client) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			HandleMessage(client, evt)

		case *events.QR:
			CurrentQR = v.Codes[0]
			fmt.Println("QR Code gerado! Abra o painel web para escanear.")

		case *events.Connected:
			if client.IsLoggedIn() {
				IsConnected = true
				CurrentQR = ""
				fmt.Println("✅ Bot conectado ao WhatsApp com sucesso!")
			}

		case *events.Disconnected:
			IsConnected = false
			fmt.Println("❌ Bot desconectado do WhatsApp (queda de rede). Tentando reconexão automática...")

		case *events.LoggedOut:
			IsConnected = false
			CurrentQR = ""
			fmt.Println("❌ O bot foi deslogado do WhatsApp pelo celular. Limpando credenciais locais...")
			
			go func() {
				time.Sleep(1 * time.Second)
				client.Disconnect()
				_ = client.Store.Delete(context.Background())

				fmt.Println("🔄 Recriando cliente WhatsApp após logout...")
				err := StartWhatsApp(context.Background())
				if err != nil {
					fmt.Printf("🚨 Erro ao reiniciar cliente WhatsApp pós-logout: %v\n", err)
				}
			}()
		}
	}
}