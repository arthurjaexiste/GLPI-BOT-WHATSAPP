package whatsapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"bot-glpi/internal/config"
	"bot-glpi/internal/state"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"golang.org/x/text/unicode/norm"
)

// matchOptionHash compara o hash recebido do WhatsApp com o texto da opção,
// garantindo compatibilidade com normalizações Unicode NFC/NFD e espaços extras
// (crítico para dispositivos iOS/macOS que decompõem caracteres acentuados como 'Não' e 'Impressoras').
func matchOptionHash(selectedHash []byte, optionText string) bool {
	// 1. Comparação direta original
	hash := sha256.Sum256([]byte(optionText))
	if bytes.Equal(selectedHash, hash[:]) {
		return true
	}

	// 2. Comparação direta sem espaços nas pontas
	trimmed := strings.TrimSpace(optionText)
	hashTrimmed := sha256.Sum256([]byte(trimmed))
	if bytes.Equal(selectedHash, hashTrimmed[:]) {
		return true
	}

	// 3. Comparação usando NFC (Normalized Form Composition)
	nfcStr := norm.NFC.String(optionText)
	hashNFC := sha256.Sum256([]byte(nfcStr))
	if bytes.Equal(selectedHash, hashNFC[:]) {
		return true
	}
	hashNFCTrimmed := sha256.Sum256([]byte(strings.TrimSpace(nfcStr)))
	if bytes.Equal(selectedHash, hashNFCTrimmed[:]) {
		return true
	}

	// 4. Comparação usando NFD (Normalized Form Decomposition)
	nfdStr := norm.NFD.String(optionText)
	hashNFD := sha256.Sum256([]byte(nfdStr))
	if bytes.Equal(selectedHash, hashNFD[:]) {
		return true
	}
	hashNFDTrimmed := sha256.Sum256([]byte(strings.TrimSpace(nfdStr)))
	if bytes.Equal(selectedHash, hashNFDTrimmed[:]) {
		return true
	}

	return false
}

func HandlePollUpdate(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string) {
	fmt.Printf("ℹ️ [POLL] Recebido voto de enquete do remetente: %s\n", sender)
	vote, err := client.DecryptPollVote(ctx, v)
	if err != nil {
		fmt.Printf("🚨 [POLL] Erro ao descriptografar voto de enquete: %v\n", err)
		return
	}
	if len(vote.SelectedOptions) == 0 {
		fmt.Printf("⚠️ [POLL] Voto recebido, mas nenhuma opção selecionada.\n")
		return
	}
	
	selectedHash := vote.SelectedOptions[0]
	fmt.Printf("ℹ️ [POLL] Voto descriptografado com sucesso. Hash selecionado: %x\n", selectedHash)

	// 1. CHECA SE FOI A ENQUETE DE ASSUMIR ATENDIMENTO (SUPORTE)
	agentsStr := config.GetConfig().SupportAgents
	var supportAgents []string
	if agentsStr != "" {
		parts := strings.Split(agentsStr, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				supportAgents = append(supportAgents, p)
			}
		}
	} else {
		supportAgents = []string{}
	}

	var nomeAgente string
	foundAgent := false
	for _, agent := range supportAgents {
		if matchOptionHash(selectedHash, agent) {
			nomeAgente = agent
			foundAgent = true
			break
		}
	}

	if foundAgent {
		state.Mu.Lock()
		state.ActiveAgentName = nomeAgente
		activeUserFull := state.ActiveLiveChatUser
		state.Mu.Unlock()

		if activeUserFull != "" {
			userJID, _ := types.ParseJID(activeUserFull)
			sendTextMessage(ctx, client, userJID, formatarMensagem(config.GetConfig().MsgSuporteAssumido, map[string]string{"agente": nomeAgente}))
			
			supportJID := types.NewJID(getSupportNumber(), types.DefaultUserServer)
			sendTextMessage(ctx, client, supportJID, fmt.Sprintf("✅ Você assumiu o atendimento como *%s*.\n\nPara responder o usuário, comece a mensagem com (!).\nPara finalizar, digite *#encerrar*.", nomeAgente))
		} else {
			supportJID := types.NewJID(getSupportNumber(), types.DefaultUserServer)
			sendTextMessage(ctx, client, supportJID, "⚠️ *Aviso do Sistema:* Não há nenhum atendimento pendente para assumir no momento.")
		}
		return
	}

	// 2. ENQUETES DE ROTINA DO USUÁRIO
	isSim := matchOptionHash(selectedHash, "Sim")
	isNao := matchOptionHash(selectedHash, "Não")

	state.Mu.Lock()
	currentStep := uState.Step
	state.Mu.Unlock()

	cfg := config.GetConfig()

	switch currentStep {
	case 30: 
		if isSim {
			state.Mu.Lock(); nomeCompleto := state.Names[sender]; primeiroNome := strings.Split(nomeCompleto, " ")[0]; state.Mu.Unlock()
			sendTextMessage(ctx, client, v.Info.Chat, fmt.Sprintf("Certo, %s! 😎", primeiroNome))
			time.Sleep(500 * time.Millisecond)
			SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
		} else if isNao {
			state.Mu.Lock(); delete(state.Names, sender); delete(state.UserIDs, sender); uState.Step = 1; state.Mu.Unlock()
			sendTextMessage(ctx, client, v.Info.Chat, "Sem problemas! Para começarmos, como você se chama? (Pode digitar seu nome completo)")
		}

	case 40: 
		if isSim {
			state.Mu.Lock(); uState.Step = 41; state.Mu.Unlock()
			sendTextMessage(ctx, client, v.Info.Chat, cfg.MsgEnviarDocumentos)
		} else if isNao {
			node, found := FindNodeByID(uState.CurrentNodeID)
			askImages := true
			if found {
				askImages = node.GetAskImages()
			}
			if askImages {
				state.Mu.Lock(); uState.Step = 35; state.Mu.Unlock()
				pollMsg := client.BuildPollCreation(cfg.MsgEnqueteFotos, []string{"Sim", "Não"}, 1)
				_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
			} else {
				FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
			}
		}

	case 42: 
		if isSim {
			node, found := FindNodeByID(uState.CurrentNodeID)
			askImages := true
			if found {
				askImages = node.GetAskImages()
			}
			if askImages {
				state.Mu.Lock(); uState.Step = 35; state.Mu.Unlock()
				pollMsg := client.BuildPollCreation(cfg.MsgEnqueteFotosPosDocs, []string{"Sim", "Não"}, 1)
				_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
			} else {
				FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
			}
		} else if isNao {
			state.Mu.Lock(); uState.Step = 41; state.Mu.Unlock()
			sendTextMessage(ctx, client, v.Info.Chat, cfg.MsgProximoDocumento)
		}

	case 35: 
		if isSim {
			state.Mu.Lock(); uState.Step = 36; state.Mu.Unlock()
			sendTextMessage(ctx, client, v.Info.Chat, cfg.MsgEnviarFotos)
		} else if isNao {
			FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
		}

	case 37: 
		if isSim {
			FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
		} else if isNao {
			state.Mu.Lock(); uState.Step = 36; state.Mu.Unlock()
			sendTextMessage(ctx, client, v.Info.Chat, cfg.MsgProximaFoto)
		}

	case 51: 
		if isSim {
			state.Mu.Lock(); uState.Step = 52; state.Mu.Unlock()
			sendTextMessage(ctx, client, v.Info.Chat, "✍️ Por favor, digite a mensagem que deseja adicionar ao chamado:")
		} else if isNao {
			sendTextMessage(ctx, client, v.Info.Chat, "Tudo bem! Agradecemos o contato. O acompanhamento deste chamado foi encerrado. Quando precisar, é só mandar uma nova mensagem! 🚀")
			
			state.Mu.Lock()
			uState.Step = -1
			uState.ActiveTicketID = 0
			uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
			state.Mu.Unlock()
		}

	case 11: 
		tratarVotoNomeSistema(ctx, client, v, uState, sender, selectedHash)

	case 1000:
		tratarVotoFluxoDinamico(ctx, client, v, uState, sender, selectedHash)
	}
}

func tratarVotoNomeSistema(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, selectedHash []byte) {
	var opcaoSelecionada string; var idSelecionado int
	state.Mu.Lock(); opcoesSalvas := uState.PollOptions; idsSalvos := uState.PollIDs; state.Mu.Unlock()

	for i, opcao := range opcoesSalvas {
		if matchOptionHash(selectedHash, opcao) {
			opcaoSelecionada = opcao 
			if opcao != "Nenhum desses" && i < len(idsSalvos) { idSelecionado = idsSalvos[i] }
			break
		}
	}

	if opcaoSelecionada == "" { return } 
	if opcaoSelecionada == "Nenhum desses" {
		state.Mu.Lock(); uState.Step = 12; state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, "Sem problemas! Por favor, digite o seu *Nome e Sobrenome* para registrarmos no chamado:")
		return
	}

	primeiroNome := strings.Split(opcaoSelecionada, " ")[0]
	state.Mu.Lock(); state.Names[sender] = opcaoSelecionada; state.UserIDs[sender] = idSelecionado; state.Mu.Unlock()
	
	sendTextMessage(ctx, client, v.Info.Chat, fmt.Sprintf("Certo, %s! 😎", primeiroNome))
	time.Sleep(500 * time.Millisecond)

	SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
}

func SendRootFlowPoll(ctx context.Context, client *whatsmeow.Client, jid types.JID, uState *state.UserState) {
	root := GetFlowConfig()
	var options []string
	for _, child := range root.Children {
		options = append(options, child.Title)
	}

	state.Mu.Lock()
	uState.CurrentNodeID = "root"
	uState.Step = 1000
	state.Mu.Unlock()

	pollMsg := client.BuildPollCreation("Como posso te ajudar hoje?", options, 1)
	_, _ = sendMessage(ctx, client, jid, pollMsg)
}

func tratarVotoFluxoDinamico(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, selectedHash []byte) {
	state.Mu.Lock()
	currentNodeID := uState.CurrentNodeID
	state.Mu.Unlock()

	if currentNodeID == "" {
		currentNodeID = "root"
	}

	fmt.Printf("ℹ️ [FLUXO] Iniciando tratarVotoFluxoDinamico para %s. currentNodeID: %q\n", sender, currentNodeID)

	// 1. VERIFICA SE O USUÁRIO CLICOU EM VOLTAR
	if matchOptionHash(selectedHash, "⬅️ Voltar") {
		fmt.Printf("ℹ️ [FLUXO] Usuário %s clicou em voltar a partir do nó %q\n", sender, currentNodeID)
		parent, found := FindParentNodeByID(currentNodeID)
		if found && parent.ID != "" {
			var options []string
			for _, child := range parent.Children {
				options = append(options, child.Title)
			}
			
			state.Mu.Lock()
			uState.CurrentNodeID = parent.ID
			uState.Step = 1000
			state.Mu.Unlock()

			var pollMsg *waE2E.Message
			if parent.ID == "root" {
				pollMsg = client.BuildPollCreation("Como posso te ajudar hoje?", options, 1)
			} else {
				if parent.GetShowBackButton() {
					options = append(options, "⬅️ Voltar")
				}
				pollMsg = client.BuildPollCreation(fmt.Sprintf("Qual o problema com %s?", parent.Title), options, 1)
			}
			_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
		} else {
			SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
		}
		return
	}

	currentNode, found := FindNodeByID(currentNodeID)
	if !found {
		fmt.Printf("🚨 [FLUXO] Nó corrente %q não encontrado no fluxo para %s!\n", currentNodeID, sender)
		SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
		return
	}

	fmt.Printf("ℹ️ [FLUXO] Nó corrente %q encontrado. Procurando correspondência para o hash %x entre %d filhos...\n", currentNode.ID, selectedHash, len(currentNode.Children))
	var selectedChild FlowNode
	foundChild := false
	for _, child := range currentNode.Children {
		matched := matchOptionHash(selectedHash, child.Title)
		fmt.Printf("   - Testando filho: ID=%s, Title=%q, Type=%s -> Matched: %t\n", child.ID, child.Title, child.Type, matched)
		if matched {
			selectedChild = child
			foundChild = true
			break
		}
	}

	if !foundChild {
		fmt.Printf("⚠️ [FLUXO] Nenhuma opção correspondente encontrada para o hash %x no nó %q para %s\n", selectedHash, currentNodeID, sender)
		return
	}

	fmt.Printf("✅ [FLUXO] Encontrado filho selecionado: ID=%s, Title=%q, Type=%s\n", selectedChild.ID, selectedChild.Title, selectedChild.Type)

	switch selectedChild.Type {
	case NodeMenu:
		if len(selectedChild.Children) == 0 {
			sendTextMessage(ctx, client, v.Info.Chat, "⚠️ Esta opção de menu está vazia ou em construção no momento.")
			time.Sleep(1 * time.Second)
			SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
			return
		}

		var options []string
		for _, child := range selectedChild.Children {
			options = append(options, child.Title)
		}
		
		// Adiciona a opção de Voltar para submenus dinâmicos se configurado para garantir
		// navegação amigável e que a enquete tenha pelo menos 2 opções (requisito do WhatsApp)
		if selectedChild.GetShowBackButton() {
			options = append(options, "⬅️ Voltar")
		}
		
		state.Mu.Lock()
		uState.CurrentNodeID = selectedChild.ID
		uState.Step = 1000
		state.Mu.Unlock()

		pollMsg := client.BuildPollCreation(fmt.Sprintf("Qual o problema com %s?", selectedChild.Title), options, 1)
		_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)

	case NodeText:
		sendTextMessage(ctx, client, v.Info.Chat, selectedChild.Content)
		
		time.Sleep(1 * time.Second)
		SendRootFlowPoll(ctx, client, v.Info.Chat, uState)

	case NodeHumanSupport:
		iniciarChatAoVivo(ctx, client, v.Info.Chat, sender)

	case NodeGLPIStatus:
		state.Mu.Lock()
		uState.Step = 50
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, "Para buscar o status, por favor digite apenas o **número do chamado** (exemplo: 1234):")

	case NodeGLPITicket:
		state.Mu.Lock()
		uState.CategoryID = selectedChild.GLPIID
		uState.SubCategory = selectedChild.Title
		uState.CurrentNodeID = selectedChild.ID
		uState.Step = 2
		state.Mu.Unlock()

		sendTextMessage(ctx, client, v.Info.Chat, fmt.Sprintf("Certo, problema *%s* anotado!\nAgora, por favor, digite um **título curto** para o problema:", selectedChild.Title))
	}
}