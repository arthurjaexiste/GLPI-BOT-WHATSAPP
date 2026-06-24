package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"bot-glpi/internal/config"
	"bot-glpi/internal/glpi"
	"bot-glpi/internal/state"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// ─── Saudação e identificação do usuário ──────────────────────────────────────

// processarSaudacaoInicial exibe a mensagem de boas-vindas e inicia o fluxo de
// identificação do usuário (novo ou retornante).
func processarSaudacaoInicial(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string) {
	state.Mu.Lock()
	nomeCompleto, jaConhece := state.Names[sender]

	if time.Since(uState.LastGreetingTime) <= 30*time.Second {
		state.Mu.Unlock()
		return
	}

	uState.InvalidAttempts = 0

	if jaConhece {
		primeiroNome := strings.Split(nomeCompleto, " ")[0]
		uState.Step = 30
		state.Mu.Unlock()

		sendTextMessage(ctx, client, v.Info.Chat, formatarMensagem(config.GetConfig().MsgUsuarioExistente, nil))
		time.Sleep(500 * time.Millisecond)

		pollMsg := client.BuildPollCreation(fmt.Sprintf("Ainda estou falando com *%s*?", primeiroNome), []string{"Sim", "Não"}, 1)
		_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
	} else {
		uState.Step = 1
		state.Mu.Unlock()

		sendTextMessage(ctx, client, v.Info.Chat, formatarMensagem(config.GetConfig().MsgNovoUsuario, nil))
	}
}

// processarBuscaNomeGLPI busca o usuário no GLPI pelo nome digitado e apresenta
// as opções encontradas em uma enquete.
func processarBuscaNomeGLPI(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender, text string) {
	sendTextMessage(ctx, client, v.Info.Chat, "🔍 Aguarde, estou buscando o seu cadastro...")

	token, err := glpi.GetGLPISession()
	if err != nil {
		fmt.Printf("\n🚨 [ERRO GLPI] Falha na Sessão: %v\n\n", err)
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Ops, ocorreu um erro de conexão com o painel de chamados.")
		return
	}

	nomes, ids, errBusca := glpi.BuscarUsuariosPorNome(token, text)

	// Segunda tentativa usando apenas o primeiro nome, se a busca completa falhar
	if (errBusca != nil || len(nomes) == 0) && strings.Contains(text, " ") {
		primeiraPalavra := strings.Split(text, " ")[0]
		nomes, ids, errBusca = glpi.BuscarUsuariosPorNome(token, primeiraPalavra)
	}

	if errBusca != nil || len(nomes) == 0 {
		state.Mu.Lock()
		uState.Step = 12
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, "⚠️ Não encontrei seu cadastro no sistema.\nPor favor, digite o seu *Nome e Sobrenome* completos para o chamado:")
		return
	}

	nomes = append(nomes, "Nenhum desses")

	state.Mu.Lock()
	uState.PollOptions = nomes
	uState.PollIDs = ids
	uState.Step = 11
	state.Mu.Unlock()

	pollMsg := client.BuildPollCreation("Selecione o seu nome abaixo:", nomes, 1)
	_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
}

// processarNomeManual registra o nome digitado manualmente pelo usuário.
func processarNomeManual(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender, text string) {
	primeiroNome := strings.Split(text, " ")[0]

	state.Mu.Lock()
	state.Names[sender] = text
	state.UserIDs[sender] = 0
	uState.Step = 10
	state.Mu.Unlock()

	sendTextMessage(ctx, client, v.Info.Chat, fmt.Sprintf("Certo, %s! 😎", primeiroNome))
	time.Sleep(500 * time.Millisecond)

	SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
}

// ─── Coleta de informações do chamado ─────────────────────────────────────────

// obterPromptDescricaoDinamico retorna o texto de prompt configurado para o nó
// atual do fluxo, ou um texto padrão se não houver configuração.
func obterPromptDescricaoDinamico(uState *state.UserState) string {
	if uState.CurrentNodeID != "" {
		if node, found := FindNodeByID(uState.CurrentNodeID); found && node.Type == NodeGLPITicket && node.Content != "" {
			return node.Content
		}
	}
	return "por favor, *descreva o problema detalhadamente*:"
}

// processarGravacaoTitulo valida e armazena o título do chamado.
func processarGravacaoTitulo(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, text string, imgMsg *waE2E.ImageMessage, docMsg *waE2E.DocumentMessage) {
	if !temLetra(text) && imgMsg == nil && docMsg == nil {
		sendTextMessage(ctx, client, v.Info.Chat, "⚠️ O título precisa ser um texto descritivo.")
		return
	}

	state.Mu.Lock()
	uState.Title = text
	uState.Step = 3
	state.Mu.Unlock()

	sendTextMessage(ctx, client, v.Info.Chat, "Ótimo! Agora, "+obterPromptDescricaoDinamico(uState))
}

// processarGravacaoDescricao valida e armazena a descrição do chamado, em seguida
// pergunta sobre documentos e/ou fotos conforme a configuração do nó.
func processarGravacaoDescricao(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender, text string, imgMsg *waE2E.ImageMessage, docMsg *waE2E.DocumentMessage) {
	if !descricaoValida(text) && imgMsg == nil && docMsg == nil {
		state.Mu.Lock()
		uState.InvalidAttempts++
		tentativas := uState.InvalidAttempts
		state.Mu.Unlock()

		if tentativas >= 4 {
			sendTextMessage(ctx, client, v.Info.Chat, "🚫 Muitas tentativas inválidas. Atendimento cancelado.")
			resetarEstadoUsuario(sender, uState)
		} else {
			sendTextMessage(ctx, client, v.Info.Chat, "⚠️ Por favor, escreva uma descrição clara do seu problema (mínimo 10 caracteres).")
		}
		return
	}

	node, found := FindNodeByID(uState.CurrentNodeID)
	askImages := true
	askDocs := true
	if found {
		askImages = node.GetAskImages()
		askDocs = node.GetAskDocs()
	}

	state.Mu.Lock()
	uState.Description = text
	uState.Images = nil
	uState.Docs = nil
	switch {
	case askDocs:
		uState.Step = 40
	case askImages:
		uState.Step = 35
	default:
		uState.Step = -1
	}
	state.Mu.Unlock()

	sendTextMessage(ctx, client, v.Info.Chat, "Problema anotado! 📝")
	time.Sleep(500 * time.Millisecond)

	cfg := config.GetConfig()
	switch {
	case askDocs:
		pollMsg := client.BuildPollCreation(cfg.MsgEnqueteDocumentos, []string{"Sim", "Não"}, 1)
		_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
	case askImages:
		pollMsg := client.BuildPollCreation(cfg.MsgEnqueteFotos, []string{"Sim", "Não"}, 1)
		_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
	default:
		FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
	}
}

// ─── Acompanhamento de chamado existente ──────────────────────────────────────

// processarBuscaChamadoInfo busca as informações de um chamado pelo ID e pergunta
// ao usuário se deseja adicionar uma nova mensagem.
func processarBuscaChamadoInfo(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender, text string) {
	ticketID, err := strconv.Atoi(text)
	if err != nil {
		sendTextMessage(ctx, client, v.Info.Chat, "⚠️ Por favor, digite *apenas números*. Qual o ID do chamado?")
		return
	}

	sendTextMessage(ctx, client, v.Info.Chat, "🔍 Buscando informações do chamado...")

	token, err := glpi.GetGLPISession()
	if err != nil {
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Falha ao conectar no sistema.")
		return
	}

	statusStr, statusInt, titulo, err := glpi.BuscarChamado(token, ticketID)
	if err != nil {
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Não encontrei nenhum chamado com esse número ou você não tem permissão.")
		time.Sleep(1 * time.Second)
		SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
		return
	}

	sendTextMessage(ctx, client, v.Info.Chat, fmt.Sprintf("📋 *Chamado #%d*\n\n*Título:* %s\n*Status:* %s", ticketID, titulo, statusStr))
	time.Sleep(1 * time.Second)

	if chamadoFinalizado(statusInt, statusStr) {
		sendTextMessage(ctx, client, v.Info.Chat, "🔒 Este chamado já está finalizado (solucionado/fechado) e não aceita novas interações.")
		time.Sleep(1 * time.Second)
		sendTextMessage(ctx, client, v.Info.Chat, "Agradecemos o contato. Quando precisar de algo, envie uma nova mensagem para recomeçar! 🚀")

		state.Mu.Lock()
		uState.Step = -1
		uState.ActiveTicketID = 0
		uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
		state.Mu.Unlock()
	} else {
		state.Mu.Lock()
		uState.ActiveTicketID = ticketID
		uState.Step = 51
		state.Mu.Unlock()

		pollMsg := client.BuildPollCreation("Deseja adicionar uma nova mensagem para o chamado?", []string{"Sim", "Não"}, 1)
		_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
	}
}

// chamadoFinalizado retorna true se o chamado está em status de solucionado/fechado.
func chamadoFinalizado(statusInt int, statusStr string) bool {
	if statusInt == 5 || statusInt == 6 {
		return true
	}
	lower := strings.ToLower(statusStr)
	keywords := []string{"solucionado", "fechado", "fechada", "resolvido", "resolvida",
		"concluido", "concluído", "finalizado", "finalizada", "encerrado", "encerrada",
		"solved", "closed", "resolu", "clos"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// processarNovaMensagemChamado envia a mensagem digitada como followup do chamado ativo.
func processarNovaMensagemChamado(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, text string) {
	sendTextMessage(ctx, client, v.Info.Chat, "Aguarde, enviando mensagem para o chamado...")

	state.Mu.Lock()
	ticketID := uState.ActiveTicketID
	state.Mu.Unlock()

	token, err := glpi.GetGLPISession()
	if err != nil {
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Falha ao conectar no sistema.")
		return
	}

	if err := glpi.AdicionarMensagemChamado(token, ticketID, text); err != nil {
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Ocorreu um erro ao adicionar a mensagem. Tente novamente mais tarde.")
	} else {
		sendTextMessage(ctx, client, v.Info.Chat, "✅ Mensagem adicionada com sucesso no chamado!")
	}

	time.Sleep(1 * time.Second)
	sendTextMessage(ctx, client, v.Info.Chat, "Agradecemos o contato! O acompanhamento deste chamado foi encerrado. Quando precisar de algo, é só chamar novamente! 🚀")

	state.Mu.Lock()
	uState.Step = -1
	uState.ActiveTicketID = 0
	uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
	state.Mu.Unlock()
}

// ─── Recebimento de mídias e anexos ──────────────────────────────────────────

// processarMidiasEAnexos trata o envio de documentos e fotos durante a abertura
// do chamado, controlando o fluxo de passos conforme as mídias chegam.
func processarMidiasEAnexos(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender, textLower string, imgMsg *waE2E.ImageMessage, docMsg *waE2E.DocumentMessage) {
	cfg := config.GetConfig()

	if docMsg != nil {
		processarDocumento(ctx, client, v, uState, docMsg, cfg)
		return
	}

	if imgMsg != nil {
		processarImagem(ctx, client, v, uState, imgMsg, cfg)
		return
	}

	// Texto de confirmação: finaliza ou avança para fotos
	if textLower == "pronto" || textLower == "sim" {
		processarConfirmacaoMidia(ctx, client, v, uState, sender, cfg)
	}
}

func processarDocumento(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, docMsg *waE2E.DocumentMessage, cfg config.Config) {
	docBytes, err := client.Download(ctx, docMsg)
	if err != nil {
		return
	}

	fileName := docMsg.GetFileName()
	if fileName == "" {
		fileName = "documento_whatsapp.pdf"
	}

	state.Mu.Lock()
	uState.Docs = append(uState.Docs, state.Doc{Bytes: docBytes, Name: fileName})
	stepAnterior := uState.Step
	uState.Step = 42
	state.Mu.Unlock()

	if stepAnterior != 42 {
		pollMsg := client.BuildPollCreation(cfg.MsgEnqueteConfirmarDocumentos, []string{"Sim", "Não"}, 1)
		_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
	} else {
		sendTextMessage(ctx, client, v.Info.Chat, cfg.MsgDocumentoAdicionado)
	}
}

func processarImagem(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, imgMsg *waE2E.ImageMessage, cfg config.Config) {
	imageBytes, err := client.Download(ctx, imgMsg)
	if err != nil {
		return
	}

	state.Mu.Lock()
	uState.Images = append(uState.Images, imageBytes)
	stepAnterior := uState.Step
	uState.Step = 37
	state.Mu.Unlock()

	if stepAnterior != 37 {
		pollMsg := client.BuildPollCreation(cfg.MsgEnqueteConfirmarFotos, []string{"Sim", "Não"}, 1)
		_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
	} else {
		sendTextMessage(ctx, client, v.Info.Chat, cfg.MsgFotoAdicionada)
	}
}

func processarConfirmacaoMidia(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, cfg config.Config) {
	state.Mu.Lock()
	currentStep := uState.Step
	state.Mu.Unlock()

	node, found := FindNodeByID(uState.CurrentNodeID)
	askImages := !found || node.GetAskImages()

	switch currentStep {
	case 41, 42:
		// Finalizou envio de documentos; pergunta sobre fotos (se configurado)
		if askImages {
			state.Mu.Lock()
			uState.Step = 35
			state.Mu.Unlock()
			pollMsg := client.BuildPollCreation(cfg.MsgEnqueteFotos, []string{"Sim", "Não"}, 1)
			_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
		} else {
			FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
		}

	case 36, 37:
		// Finalizou envio de fotos
		FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
	}
}

// ─── Criação e notificação do chamado ─────────────────────────────────────────

// FinalizarChamadoEAlertar cria o chamado no GLPI com todas as mídias coletadas,
// notifica o usuário e envia um alerta interno para o suporte.
func FinalizarChamadoEAlertar(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string) {
	sendTextMessage(ctx, client, v.Info.Chat, "Aguarde um momento, registrando seu chamado no sistema...")

	state.Mu.Lock()
	subCat := uState.SubCategory
	tituloChamado := fmt.Sprintf("%s - %s", state.Names[sender], uState.Title)
	if subCat != "" && subCat != "Outros" && uState.Title != subCat {
		tituloChamado += fmt.Sprintf(" (%s)", subCat)
	}

	descricaoOriginal := uState.Description
	requesterID := state.UserIDs[sender]
	nomeVisitante := state.Names[sender]
	categoriaID := uState.CategoryID
	imagensSalvas := uState.Images
	documentosSalvos := uState.Docs

	uState.Images = nil
	uState.Docs = nil
	uState.SubCategory = ""
	state.Mu.Unlock()

	descricaoHTML := montarDescricaoHTML(descricaoOriginal, imagensSalvas)

	token, err := glpi.GetGLPISession()
	if err != nil {
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Falha de comunicação com o servidor GLPI. Verifique os logs.")
		return
	}

	// Cria usuário visitante se necessário
	if requesterID == 0 {
		if novoID, err := glpi.CriarUsuarioVisitante(token, nomeVisitante); err == nil {
			requesterID = novoID
			state.Mu.Lock()
			state.UserIDs[sender] = novoID
			state.Mu.Unlock()
		}
	}

	ticketID, err := glpi.CriarChamado(token, tituloChamado, descricaoHTML, 3, requesterID, categoriaID)
	if err != nil {
		fmt.Printf("🚨 [TICKET] Erro ao criar chamado no GLPI para %s (%s): %v\n", nomeVisitante, sender, err)
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Ocorreu um erro ao registrar o ticket no GLPI.")
		return
	}

	fmt.Printf("✅ [TICKET] Chamado #%d criado para %s (%s) | Categoria: %s (ID: %d)\n", ticketID, nomeVisitante, sender, subCat, categoriaID)

	// Salva no histórico do painel web
	salvarHistoricoTicket(ticketID, tituloChamado, nomeVisitante)

	// Faz upload de anexos
	for _, doc := range documentosSalvos {
		_ = glpi.AnexarDocumento(token, ticketID, doc.Bytes, doc.Name)
	}
	for i, imgBytes := range imagensSalvas {
		_ = glpi.AnexarDocumento(token, ticketID, imgBytes, fmt.Sprintf("foto_whatsapp_%d.jpeg", i+1))
	}

	// Notifica o usuário
	cfg := config.GetConfig()
	notificarUsuarioTicketCriado(ctx, client, v.Info.Chat, ticketID, tituloChamado, cfg)

	// Notifica o suporte
	if cfg.TelefoneNotificacao != "" {
		notificarSuporteTicketCriado(ctx, client, ticketID, tituloChamado, nomeVisitante, categoriaID, cfg)
	}

	state.Mu.Lock()
	uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
	uState.Step = -1
	state.Mu.Unlock()
}

// montarDescricaoHTML converte a descrição em HTML e embute as imagens em Base64.
func montarDescricaoHTML(descricao string, imagens [][]byte) string {
	html := "<div>" + strings.ReplaceAll(descricao, "\n", "<br>") + "</div>"

	if len(imagens) > 0 {
		html += "<br><br><b>Fotos enviadas via WhatsApp:</b><br>"
		for _, imgBytes := range imagens {
			b64 := base64.StdEncoding.EncodeToString(imgBytes)
			html += fmt.Sprintf(`<br><img src="data:image/jpeg;base64,%s" style="max-width: 100%%; border: 1px solid #ccc; margin-top: 10px;"><br>`, b64)
		}
	}

	return html
}

// salvarHistoricoTicket persiste o ticket no banco de dados do painel web, se disponível.
func salvarHistoricoTicket(ticketID int, titulo, solicitante string) {
	if webDB == nil {
		return
	}
	_, err := webDB.Exec(
		"INSERT INTO tickets_history (ticket_id, title, requester) VALUES (?, ?, ?)",
		strconv.Itoa(ticketID), titulo, solicitante,
	)
	if err != nil {
		fmt.Printf("🚨 [DATABASE] Erro ao salvar ticket no histórico: %v\n", err)
	}
}

// notificarUsuarioTicketCriado envia a confirmação do ticket ao usuário.
func notificarUsuarioTicketCriado(ctx context.Context, client *whatsmeow.Client, chatJID types.JID, ticketID int, titulo string, cfg config.Config) {
	ticketIDStr := strconv.Itoa(ticketID)

	if IsOutsideWorkingHours() && cfg.MsgAusencia != "" {
		msg := formatarMensagem(cfg.MsgAusencia, map[string]string{
			"ticket_id":    ticketIDStr,
			"ticket_title": titulo,
			"inicio":       cfg.WorkingHoursStart,
			"fim":          cfg.WorkingHoursEnd,
		})
		sendTextMessage(ctx, client, chatJID, msg)
		return
	}

	sendTextMessage(ctx, client, chatJID, formatarMensagem(cfg.MsgTicketCriado, map[string]string{
		"ticket_id":    ticketIDStr,
		"ticket_title": titulo,
	}))
}

// notificarSuporteTicketCriado envia o alerta interno de novo chamado para o suporte.
func notificarSuporteTicketCriado(ctx context.Context, client *whatsmeow.Client, ticketID int, titulo, solicitante string, categoriaID int, cfg config.Config) {
	targetJID := types.NewJID(cfg.TelefoneNotificacao, types.DefaultUserServer)
	ticketIDStr := strconv.Itoa(ticketID)
	linkTicket := obterLinkTicketGLPI(ticketIDStr)

	textoAlerta := fmt.Sprintf(
		"🔔 *NOVO CHAMADO VIA WHATSAPP*\n\n"+
			"📌 *Ticket:* #%s %s\n"+
			"👤 *Solicitante:* %s\n"+
			"📁 *Categoria ID:* %d\n"+
			"📝 *Título:* %s\n\n"+
			"⚠️ _Abra o painel do GLPI para iniciar as tratativas._",
		ticketIDStr, linkTicket, solicitante, categoriaID, titulo,
	)

	sendTextMessage(ctx, client, targetJID, textoAlerta)
}

// ─── Validações ───────────────────────────────────────────────────────────────

// temLetra verifica se uma string contém pelo menos uma letra.
func temLetra(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// descricaoValida verifica se a descrição tem conteúdo mínimo aceitável
// (evita mensagens com caracteres repetidos ou textos muito curtos).
func descricaoValida(text string) bool {
	if !temLetra(text) || len([]rune(text)) <= 10 {
		return false
	}
	for _, r := range text {
		if strings.Count(text, strings.Repeat(string(r), 4)) > 0 {
			return false
		}
	}
	return true
}