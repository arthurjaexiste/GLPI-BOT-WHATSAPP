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

func obterPromptDescricaoDinamico(uState *state.UserState) string {
	if uState.CurrentNodeID != "" {
		if node, found := FindNodeByID(uState.CurrentNodeID); found && node.Type == NodeGLPITicket && node.Content != "" {
			return node.Content
		}
	}
	return "por favor, *descreva o problema detalhadamente*:"
}

func processarSaudacaoInicial(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string) {
	state.Mu.Lock()
	nomeCompleto, jaConhece := state.Names[sender]

	if time.Since(uState.LastGreetingTime) > 30*time.Second {
		uState.InvalidAttempts = 0

		if jaConhece {
			primeiroNome := strings.Split(nomeCompleto, " ")[0]
			uState.Step = 30
			state.Mu.Unlock()

			sendTextMessage(ctx, client, v.Info.Chat, formatarMensagem(config.GetConfig().MsgUsuarioExistente, nil))
			time.Sleep(500 * time.Millisecond)

			pollMsg := client.BuildPollCreation(fmt.Sprintf("Ainda estou falando com *%s*?", primeiroNome), []string{"Sim", "Não"}, 1)
			client.SendMessage(ctx, v.Info.Chat, pollMsg)
		} else {
			uState.Step = 1
			state.Mu.Unlock()

			sendTextMessage(ctx, client, v.Info.Chat, formatarMensagem(config.GetConfig().MsgNovoUsuario, nil))
		}
	} else {
		state.Mu.Unlock()
	}
}

func processarBuscaNomeGLPI(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, text string) {
	sendTextMessage(ctx, client, v.Info.Chat, "🔍 Aguarde, estou buscando o seu cadastro...")
	token, errSessao := glpi.GetGLPISession()
	if errSessao != nil {
		fmt.Printf("\n🚨 [ERRO GLPI] Falha na Sessão: %v\n\n", errSessao)
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Ops, ocorreu um erro de conexão com o painel de chamados.")
		return
	}

	nomes, ids, errBusca := glpi.BuscarUsuariosPorNome(token, text)
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
	client.SendMessage(ctx, v.Info.Chat, pollMsg)
}

func processarNomeManual(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, text string) {
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

func processarGravacaoTitulo(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, text string, imgMsg *waE2E.ImageMessage, docMsg *waE2E.DocumentMessage) {
	hasLetter := false
	for _, r := range text {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter && imgMsg == nil && docMsg == nil {
		sendTextMessage(ctx, client, v.Info.Chat, "⚠️ O título precisa ser um texto descritivo.")
		return
	}

	state.Mu.Lock()
	uState.Title = text
	uState.Step = 3
	state.Mu.Unlock()

	prompt := obterPromptDescricaoDinamico(uState)
	sendTextMessage(ctx, client, v.Info.Chat, "Ótimo! Agora, "+prompt)
}

func processarGravacaoDescricao(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, text string, imgMsg *waE2E.ImageMessage, docMsg *waE2E.DocumentMessage) {
	isGibberish := false
	for _, r := range text {
		if strings.Count(text, strings.Repeat(string(r), 4)) > 0 {
			isGibberish = true
			break
		}
	}
	hasLetter := false
	for _, r := range text {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}

	if (isGibberish || !hasLetter || len([]rune(text)) <= 10) && imgMsg == nil && docMsg == nil {
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
	if askDocs {
		uState.Step = 40
	} else if askImages {
		uState.Step = 35
	} else {
		uState.Step = -1
	}
	state.Mu.Unlock()

	sendTextMessage(ctx, client, v.Info.Chat, "Problema anotado! 📝")
	time.Sleep(500 * time.Millisecond)

	if askDocs {
		pollMsg := client.BuildPollCreation("Você possui arquivos ou documentos (PDF, Word, Excel, etc) para enviar?", []string{"Sim", "Não"}, 1)
		client.SendMessage(ctx, v.Info.Chat, pollMsg)
	} else if askImages {
		pollMsg := client.BuildPollCreation("Você tem alguma FOTO ou PRINT do problema para enviar?", []string{"Sim", "Não"}, 1)
		client.SendMessage(ctx, v.Info.Chat, pollMsg)
	} else {
		FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
	}
}

func processarBuscaChamadoInfo(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, text string) {
	ticketID, err := strconv.Atoi(text)
	if err != nil {
		sendTextMessage(ctx, client, v.Info.Chat, "⚠️ Por favor, digite *apenas números*. Qual o ID do chamado?")
		return
	}

	sendTextMessage(ctx, client, v.Info.Chat, "🔍 Buscando informações do chamado...")
	token, errSessao := glpi.GetGLPISession()
	if errSessao == nil {
		status, statusInt, titulo, errBusca := glpi.BuscarChamado(token, ticketID)
		if errBusca != nil {
			sendTextMessage(ctx, client, v.Info.Chat, "❌ Não encontrei nenhum chamado com esse número ou você não tem permissão.")

			time.Sleep(1 * time.Second)
			SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
		} else {
			mensagemDetalhe := fmt.Sprintf("📋 *Chamado #%d*\n\n*Título:* %s\n*Status:* %s", ticketID, titulo, status)
			sendTextMessage(ctx, client, v.Info.Chat, mensagemDetalhe)

			time.Sleep(1 * time.Second)
			
			// Se o chamado estiver Solucionado (5) ou Fechado (6), ou o texto do status indicar isso, impede novas mensagens e enquetes
			statusLower := strings.ToLower(status)
			if statusInt == 5 || statusInt == 6 || strings.Contains(statusLower, "solucionado") || strings.Contains(statusLower, "fechado") {
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
				client.SendMessage(ctx, v.Info.Chat, pollMsg)
			}
		}
	} else {
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Falha ao conectar no sistema.")
	}
}

func processarNovaMensagemChamado(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, text string) {
	sendTextMessage(ctx, client, v.Info.Chat, "Aguarde, enviando mensagem para o chamado...")

	state.Mu.Lock()
	ticketID := uState.ActiveTicketID
	state.Mu.Unlock()

	token, errSessao := glpi.GetGLPISession()
	if errSessao != nil {
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Falha ao conectar no sistema.")
		return
	}

	err := glpi.AdicionarMensagemChamado(token, ticketID, text)
	if err != nil {
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

func processarMidiasEAnexos(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, textLower string, imgMsg *waE2E.ImageMessage, docMsg *waE2E.DocumentMessage) {
	if docMsg != nil {
		docBytes, err := client.Download(ctx, docMsg)
		if err == nil {
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
				pollMsg := client.BuildPollCreation("📄 Arquivo recebido! Já terminou de enviar seus documentos?", []string{"Sim", "Não"}, 1)
				client.SendMessage(ctx, v.Info.Chat, pollMsg)
			} else {
				sendTextMessage(ctx, client, v.Info.Chat, "✅ Mais um documento adicionado à lista!")
			}
		}
		return
	}

	if imgMsg != nil {
		imageBytes, err := client.Download(ctx, imgMsg)
		if err == nil {
			state.Mu.Lock()
			uState.Images = append(uState.Images, imageBytes)
			stepAnterior := uState.Step
			uState.Step = 37
			state.Mu.Unlock()
			if stepAnterior != 37 {
				pollMsg := client.BuildPollCreation("📸 Foto recebida! Já terminou de enviar suas fotos?", []string{"Sim", "Não"}, 1)
				client.SendMessage(ctx, v.Info.Chat, pollMsg)
			} else {
				sendTextMessage(ctx, client, v.Info.Chat, "✅ Mais uma foto adicionada à lista!")
			}
		}
		return
	}

	if textLower == "pronto" || textLower == "sim" {
		state.Mu.Lock()
		currentStep := uState.Step
		state.Mu.Unlock()
		if currentStep == 41 || currentStep == 42 {
			node, found := FindNodeByID(uState.CurrentNodeID)
			askImages := true
			if found {
				askImages = node.GetAskImages()
			}
			if askImages {
				state.Mu.Lock()
				uState.Step = 35
				state.Mu.Unlock()
				pollMsg := client.BuildPollCreation("Você tem alguma FOTO ou PRINT do problema para enviar?", []string{"Sim", "Não"}, 1)
				client.SendMessage(ctx, v.Info.Chat, pollMsg)
			} else {
				FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
			}
		} else if currentStep == 36 || currentStep == 37 {
			FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
		}
	}
}

func FinalizarChamadoEAlertar(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string) {
	sendTextMessage(ctx, client, v.Info.Chat, "Aguarde um momento, registrando seu chamado no sistema...")

	state.Mu.Lock()
	subCat := uState.SubCategory
	tituloChamado := fmt.Sprintf("%s - %s", state.Names[sender], uState.Title)

	if subCat != "" && subCat != "Outros" {
		if uState.Title != subCat {
			tituloChamado += fmt.Sprintf(" (%s)", subCat)
		}
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

	descricaoHTML := "<div>" + strings.ReplaceAll(descricaoOriginal, "\n", "<br>") + "</div>"

	if len(imagensSalvas) > 0 {
		descricaoHTML += "<br><br><b>Fotos enviadas via WhatsApp:</b><br>"
		for _, imgBytes := range imagensSalvas {
			b64 := base64.StdEncoding.EncodeToString(imgBytes)
			descricaoHTML += fmt.Sprintf("<br><img src=\"data:image/jpeg;base64,%s\" style=\"max-width: 100%%; border: 1px solid #ccc; margin-top: 10px;\"><br>", b64)
		}
	}

	token, errSessao := glpi.GetGLPISession()
	if errSessao != nil {
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Falha de comunicação com o servidor GLPI. Verifique os logs.")
		return
	}

	if requesterID == 0 {
		novoID, errNovoUser := glpi.CriarUsuarioVisitante(token, nomeVisitante)
		if errNovoUser == nil {
			requesterID = novoID
			state.Mu.Lock()
			state.UserIDs[sender] = novoID
			state.Mu.Unlock()
		}
	}

	ticketID, err := glpi.CriarChamado(token, tituloChamado, descricaoHTML, 3, requesterID, categoriaID)

	if err == nil {
		for _, doc := range documentosSalvos {
			_ = glpi.AnexarDocumento(token, ticketID, doc.Bytes, doc.Name)
		}
		for i, imgBytes := range imagensSalvas {
			_ = glpi.AnexarDocumento(token, ticketID, imgBytes, fmt.Sprintf("foto_whatsapp_%d.jpeg", i+1))
		}

		sendTextMessage(ctx, client, v.Info.Chat, formatarMensagem(config.GetConfig().MsgTicketCriado, map[string]string{"ticket_id": strconv.Itoa(ticketID), "ticket_title": tituloChamado}))

		numeroTI := config.GetConfig().TelefoneNotificacao
		if numeroTI != "" {
			targetJID := types.NewJID(numeroTI, types.DefaultUserServer)
			textoAlerta := fmt.Sprintf(
				"🔔 *NOVO CHAMADO VIA WHATSAPP*\n\n"+
					"📌 *Ticket:* #%d\n"+
					"👤 *Solicitante:* %s\n"+
					"📁 *Categoria ID:* %d\n"+
					"📝 *Título:* %s\n\n"+
					"⚠️ _Abra o painel do GLPI para iniciar as tratativas._",
				ticketID, nomeVisitante, categoriaID, tituloChamado,
			)
			sendTextMessage(ctx, client, targetJID, textoAlerta)
		}

		state.Mu.Lock()
		uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
		uState.Step = -1
		state.Mu.Unlock()
	} else {
		sendTextMessage(ctx, client, v.Info.Chat, "❌ Ocorreu um erro ao registrar o ticket no GLPI.")
	}
}