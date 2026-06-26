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

// ─── Estado global compartilhado ─────────────────────────────────────────────

var (
	CurrentQR       string
	IsConnected     bool
	ClientMu        sync.Mutex
	GlobalClient    *whatsmeow.Client
	GlobalContainer *sqlstore.Container

	// Banco de dados e sessões do painel web
	webDB      *sql.DB
	sessions   = make(map[string]time.Time)
	sessionsMu sync.Mutex
)

// ─── Gerenciamento de estado do usuário ───────────────────────────────────────

// resetarEstadoUsuario volta o usuário ao passo inicial, limpando dados parciais.
func resetarEstadoUsuario(sender string, uState *state.UserState) {
	state.Mu.Lock()
	defer state.Mu.Unlock()

	uState.Step = -1
	uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
	uState.Images = nil
	uState.Docs = nil
	uState.SubCategory = ""
	uState.ActiveTicketID = 0
}

// retrocederPasso trata o comando "*" enviado pelo usuário para navegar de volta.
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
			options := buildChildOptions(parent)
			uState.CurrentNodeID = parent.ID
			state.Mu.Unlock()
			_, _ = sendMessage(ctx, client, v.Info.Chat, buildMenuPoll(client, parent, options))
		} else {
			state.Mu.Unlock()
			SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
		}

	case 2:
		uState.Title = ""
		currentNodeID := uState.CurrentNodeID
		parent, found := FindParentNodeByID(currentNodeID)
		if found {
			options := buildChildOptions(parent)
			uState.Step = 1000
			uState.CurrentNodeID = parent.ID
			state.Mu.Unlock()
			_, _ = sendMessage(ctx, client, v.Info.Chat, buildMenuPoll(client, parent, options))
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
		sendTextMessage(ctx, client, v.Info.Chat, "Voltando... "+obterPromptDescricaoDinamico(uState))

	default:
		state.Mu.Unlock()
	}
}

// buildChildOptions extrai os títulos dos filhos de um nó para montar a enquete.
func buildChildOptions(node FlowNode) []string {
	options := make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		options = append(options, child.Title)
	}
	return options
}

// buildMenuPoll cria a enquete do WhatsApp para um nó de menu.
func buildMenuPoll(client *whatsmeow.Client, node FlowNode, options []string) *waE2E.Message {
	if node.ID == "root" {
		return client.BuildPollCreation("Como posso te ajudar hoje?", options, 1)
	}
	return client.BuildPollCreation(fmt.Sprintf("Qual o problema com %s?", node.Title), options, 1)
}

// ─── Handler principal de mensagens ───────────────────────────────────────────

// HandleMessage é o ponto de entrada para todas as mensagens recebidas pelo bot.
func HandleMessage(client *whatsmeow.Client, evt interface{}) {
	v, ok := evt.(*events.Message)
	if !ok || v.Info.IsFromMe || v.Info.IsGroup {
		return
	}

	ctx := context.Background()
	chatJID := normalizarJID(ctx, client, v.Info.Chat)
	sender := NormalizePhoneLocal(chatJID.User)
	pollUpdate := v.Message.GetPollUpdateMessage()

	rawText, imgMsg, docMsg := extrairConteudoMensagem(v)
	text := strings.TrimSpace(rawText)
	textLower := strings.ToLower(text)

	client.MarkRead(ctx, []types.MessageID{v.Info.ID}, v.Info.Timestamp, chatJID, v.Info.Sender)

	// Lê estado do usuário e da sessão de live chat de forma segura
	state.Mu.Lock()
	userName := state.Names[sender]
	userStep := -1
	if uState, exists := state.Users[sender]; exists {
		userStep = uState.Step
	}
	activeUserFull := state.ActiveLiveChatUser
	state.Mu.Unlock()

	logReceivedMessage(sender, userName, userStep, activeUserFull, text, imgMsg, docMsg, pollUpdate)

	// Salva a mensagem recebida no banco para visualização no painel
	if webDB != nil {
		senderName := userName
		if senderName == "" {
			senderName = v.Info.PushName
		}
		if senderName == "" {
			senderName = sender
		}

		msgText := text
		msgType := "text"
		if imgMsg != nil {
			msgText = "[Imagem]"
			if imgMsg.Caption != nil {
				msgText = "[Imagem] " + *imgMsg.Caption
			}
			msgType = "image"
		} else if docMsg != nil {
			msgText = "[Documento] " + docMsg.GetFileName()
			msgType = "document"
		} else if pollUpdate != nil {
			msgText = "[Voto em Enquete]"
			msgType = "poll"
		}

		_, _ = webDB.Exec(
			"INSERT INTO chat_messages (chat_jid, sender_name, sender_jid, message_text, message_type, is_from_me) VALUES (?, ?, ?, ?, ?, 0)",
			chatJID.String(), senderName, v.Info.Sender.String(), msgText, msgType,
		)
	}

	// Verifica se o remetente é o número de suporte configurado
	supportNumber := getSupportNumber()
	isSupport := phonesSufixMatch(sender, supportNumber, 8)

	// ── Comandos do suporte (mensagens iniciadas com "!") ─────────────────
	if strings.HasPrefix(text, "!") {
		handleSupportCommand(ctx, client, v, text, sender, isSupport, activeUserFull)
		return
	}

	// ── Encerramento do chat ao vivo ──────────────────────────────────────
	if textLower == "#encerrar" && activeUserFull != "" {
		activeJID, _ := types.ParseJID(activeUserFull)
		encerradoPeloSuporte := sender != activeJID.User
		encerrarChatAoVivo(ctx, client, encerradoPeloSuporte)
		return
	}

	// ── Barreira: suporte e lista negra não passam pelo fluxo normal ──────
	if isSupport || isBlacklisted(sender) {
		return
	}

	// ── Obtém (ou cria) o estado do usuário ───────────────────────────────
	state.Mu.Lock()
	uState, exists := state.Users[sender]
	if !exists {
		uState = &state.UserState{Step: -1, LastGreetingTime: time.Now().Add(-15 * time.Minute)}
		state.Users[sender] = uState
	}
	currentStep := uState.Step
	state.Mu.Unlock()

	// ── Fluxo de chat ao vivo ativo (usuário) ─────────────────────────────
	if currentStep == 100 {
		handleUserInLiveChat(ctx, client, v, uState, sender, chatJID, text, textLower, imgMsg, docMsg)
		return
	}

	// ── Usuário na fila de espera ─────────────────────────────────────────
	if currentStep == 99 {
		handleUserInQueue(ctx, client, uState, sender, chatJID, textLower, text)
		return
	}

	// ── Comandos globais de cancelamento e navegação ──────────────────────
	if textLower == "cancelar" || textLower == "sair" || text == "#" {
		if currentStep != -1 {
			resetarEstadoUsuario(sender, uState)
			sendTextMessage(ctx, client, chatJID, "✅ Atendimento cancelado. Quando precisar, envie uma nova mensagem para recomeçar! 🚀")
		}
		return
	}
	if text == "*" && currentStep != -1 {
		retrocederPasso(ctx, client, v, uState, sender)
		return
	}

	// ── Respostas de enquete ──────────────────────────────────────────────
	if pollUpdate != nil {
		HandlePollUpdate(ctx, client, v, uState, sender)
		return
	}

	// ── Recebimento de mídias/anexos ──────────────────────────────────────
	if currentStep >= 35 && currentStep <= 42 {
		processarMidiasEAnexos(ctx, client, v, uState, sender, textLower, imgMsg, docMsg)
		return
	}

	// ── Máquina de estados principal ──────────────────────────────────────
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

// ─── Handlers auxiliares do fluxo principal ───────────────────────────────────

func handleSupportCommand(ctx context.Context, client *whatsmeow.Client, v *events.Message, text, sender string, isSupport bool, activeUserFull string) {
	if activeUserFull == "" {
		if isSupport {
			sendTextMessage(ctx, client, v.Info.Chat, "⚠️ *Aviso do Sistema:* Não há nenhum usuário no chat ao vivo no momento.")
		}
		return
	}

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
}

func handleUserInLiveChat(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, chatJID types.JID, text, textLower string, imgMsg *waE2E.ImageMessage, docMsg *waE2E.DocumentMessage) {
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

	if audioMsg == nil && videoMsg == nil && stickerMsg == nil &&
		v.Message.GetImageMessage() == nil && v.Message.GetDocumentMessage() == nil {
		sendTextMessage(ctx, client, supportJID, fmt.Sprintf("👤 *%s:*\n\n%s", nome, text))
	} else {
		sendTextMessage(ctx, client, supportJID, fmt.Sprintf("👤 *%s enviou o arquivo/mídia abaixo:*", nome))
		_, _ = sendMessage(ctx, client, supportJID, v.Message)
	}
}

func handleUserInQueue(ctx context.Context, client *whatsmeow.Client, uState *state.UserState, sender string, chatJID types.JID, textLower, text string) {
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
}

// ─── Conexão e eventos do WhatsApp ────────────────────────────────────────────

// StartWhatsApp inicializa o cliente whatsmeow e conecta ao WhatsApp.
// Se não houver sessão salva, inicia o processo de geração de QR Code.
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
		return conectarComQR(ctx, client)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("erro ao conectar: %w", err)
	}

	return nil
}

// conectarComQR inicia a conexão via QR Code e publica os códigos no canal global.
func conectarComQR(ctx context.Context, client *whatsmeow.Client) error {
	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("erro ao obter canal de QR Code: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("erro ao conectar: %w", err)
	}

	go func() {
		for evt := range qrChan {
			switch evt.Event {
			case "code":
				CurrentQR = evt.Code
				IsConnected = false
				fmt.Println("⚠️  NOVO QR CODE GERADO. VEJA NO PAINEL WEB OU ESCANEIE.")
			case "success":
				IsConnected = true
				CurrentQR = ""
				fmt.Println("✅ Bot conectado ao WhatsApp via QR Code!")
			}
		}
	}()

	return nil
}

// GetEventHandler retorna o handler de eventos do whatsmeow para o cliente fornecido.
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
			go handleLogout(client)
		}
	}
}

// handleLogout aguarda um segundo, limpa as credenciais e reinicia o cliente.
func handleLogout(client *whatsmeow.Client) {
	time.Sleep(1 * time.Second)
	client.Disconnect()
	_ = client.Store.Delete(context.Background())

	fmt.Println("🔄 Recriando cliente WhatsApp após logout...")
	if err := StartWhatsApp(context.Background()); err != nil {
		fmt.Printf("🚨 Erro ao reiniciar cliente WhatsApp pós-logout: %v\n", err)
	}
}

// ─── Helpers de log ───────────────────────────────────────────────────────────

func logReceivedMessage(sender, userName string, userStep int, activeUserFull, text string, imgMsg *waE2E.ImageMessage, docMsg *waE2E.DocumentMessage, pollUpdate interface{}) {
	debugName := sender
	if userName != "" {
		debugName = fmt.Sprintf("%s (%s)", userName, sender)
	}

	if pollUpdate != nil {
		fmt.Printf("📥 [ENQUETE] Usuário %s votou em uma enquete do WhatsApp.\n", debugName)
		return
	}

	if userStep == 100 && activeUserFull != "" {
		if imgMsg != nil {
			fmt.Printf("💬 [LIVECHAT] Cliente %s em atendimento enviou uma imagem\n", debugName)
		} else if docMsg != nil {
			fmt.Printf("💬 [LIVECHAT] Cliente %s em atendimento enviou um documento (%q)\n", debugName, docMsg.GetFileName())
		} else {
			fmt.Printf("💬 [LIVECHAT] Cliente %s em atendimento enviou mensagem: %q\n", debugName, text)
		}
		return
	}

	if imgMsg != nil {
		fmt.Printf("📥 [IMAGEM] Usuário %s enviou uma imagem | Passo: %d\n", debugName, userStep)
	} else if docMsg != nil {
		fmt.Printf("📥 [DOCUMENTO] Usuário %s enviou um documento (%q) | Passo: %d\n", debugName, docMsg.GetFileName(), userStep)
	} else {
		fmt.Printf("📥 [MENSAGEM] Usuário %s enviou: %q | Passo: %d\n", debugName, text, userStep)
	}
}

// phonesSufixMatch compara dois números de telefone de forma segura,
// tratando o 9º dígito brasileiro de forma que impeça colisões entre DDDs diferentes.
func phonesSufixMatch(a, b string, n int) bool {
	// Limpa formatação e remove JID sufixos
	clean := func(p string) string {
		p = strings.Split(p, "@")[0]
		p = strings.Split(p, ":")[0]
		return strings.NewReplacer("+", "", "-", "", " ", "").Replace(p)
	}

	ensureDDI := func(p string) string {
		p = clean(p)
		if !strings.HasPrefix(p, "55") && (len(p) == 10 || len(p) == 11) {
			return "55" + p
		}
		return p
	}

	numA := ensureDDI(a)
	numB := ensureDDI(b)

	if numA == numB {
		return true
	}

	// Se ambos são brasileiros (DDI 55)
	if strings.HasPrefix(numA, "55") && strings.HasPrefix(numB, "55") {
		// Garante tamanho mínimo para extrair DDD (55 + 2 dígitos DDD = 4 caracteres)
		if len(numA) >= 4 && len(numB) >= 4 {
			// Compara o DDD (posições 2 e 3)
			if numA[2:4] != numB[2:4] {
				return false // DDDs diferentes, não são a mesma pessoa
			}
			// Compara o restante removendo o 9º dígito se presente
			restA := numA[4:]
			restB := numB[4:]
			if len(restA) == 9 && restA[0] == '9' {
				restA = restA[1:]
			}
			if len(restB) == 9 && restB[0] == '9' {
				restB = restB[1:]
			}
			return restA == restB
		}
	}

	// Fallback para comparação de sufixo padrão caso um dos números não seja brasileiro ou seja muito curto
	if len(numA) >= n && len(numB) >= n {
		return numA[len(numA)-n:] == numB[len(numB)-n:]
	}

	return numA == numB
}