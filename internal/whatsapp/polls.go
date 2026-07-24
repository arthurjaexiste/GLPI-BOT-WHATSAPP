// ============================================================================
// ARQUIVO: polls.go
// Descrição: Implementação Go (backend) para o ecossistema GLPI-BOT.
// ============================================================================

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
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"golang.org/x/text/unicode/norm"
)

// ─── Comparação de hashes de enquete ─────────────────────────────────────────

// matchOptionHash compara o hash recebido do WhatsApp com o texto de uma opção.
//
// Inclui normalizações NFC/NFD para garantir compatibilidade com dispositivos
// iOS/macOS, que decompõem caracteres acentuados (ex: "Não", "Impressoras").

// Função matchOptionHash executa a regra de negócio/rotina correspondente
func matchOptionHash(selectedHash []byte, optionText string) bool {
	// Variantes a testar: original, sem espaços nas pontas, NFC e NFD
	candidates := []string{
		optionText,
		strings.TrimSpace(optionText),
		norm.NFC.String(optionText),
		strings.TrimSpace(norm.NFC.String(optionText)),
		norm.NFD.String(optionText),
		strings.TrimSpace(norm.NFD.String(optionText)),
	}

	for _, candidate := range candidates {
		hash := sha256.Sum256([]byte(candidate))
		if bytes.Equal(selectedHash, hash[:]) {
			return true
		}
	}

	return false
}

// ─── Handler principal de enquetes ───────────────────────────────────────────

// HandlePollUpdate processa os votos recebidos em enquetes do WhatsApp.

// Função HandlePollUpdate executa a regra de negócio/rotina correspondente
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

	// Prioridade 1: enquete de assumir atendimento (enviada ao suporte)
	if tratarVotoAtribuicaoAgente(ctx, client, v, selectedHash) {
		return
	}

	// Prioridade 2: enquetes de fluxo do usuário
	isSim := matchOptionHash(selectedHash, "Sim")
	isNao := matchOptionHash(selectedHash, "Não")

	state.Mu.Lock()
	currentStep := uState.Step
	if currentStep == -1 {
		// Fallback: se o bot reiniciou ou a sessão expirou, mas o usuário respondeu à enquete do menu principal, recupera a navegação
		uState.Step = 1000
		uState.CurrentNodeID = "root"
		currentStep = 1000
	}
	state.Mu.Unlock()

	cfg := config.GetConfig()

	switch currentStep {
	case 30:
		tratarVotoConfirmacaoIdentidade(ctx, client, v, uState, sender, isSim, isNao)
	case 35:
		tratarVotoEnqueteFotos(ctx, client, v, uState, sender, isSim, isNao, cfg)
	case 37:
		tratarVotoConfirmacaoFotos(ctx, client, v, uState, sender, isSim, isNao, cfg)
	case 40:
		tratarVotoEnqueteDocumentos(ctx, client, v, uState, sender, isSim, isNao, cfg)
	case 42:
		tratarVotoConfirmacaoDocumentos(ctx, client, v, uState, sender, isSim, isNao, cfg)
	case 51:
		tratarVotoNovaMensagemChamado(ctx, client, v, uState, isSim, isNao)
	case 11:
		tratarVotoNomeSistema(ctx, client, v, uState, sender, selectedHash)
	case 1000:
		tratarVotoFluxoDinamico(ctx, client, v, uState, sender, selectedHash)
	}
}

// ─── Handlers por etapa do fluxo ─────────────────────────────────────────────

// tratarVotoAtribuicaoAgente verifica se o voto é de um atendente assumindo o chat.
// Retorna true se o voto foi processado como atribuição, false caso contrário.

// Função tratarVotoAtribuicaoAgente executa a regra de negócio/rotina correspondente
func tratarVotoAtribuicaoAgente(ctx context.Context, client *whatsmeow.Client, v *events.Message, selectedHash []byte) bool {
	supportAgents := parsearAtendentes(config.GetConfig().SupportAgents)

	for _, agent := range supportAgents {
		if matchOptionHash(selectedHash, agent) {
			state.Mu.Lock()
			state.ActiveAgentName = agent
			activeUserFull := state.ActiveLiveChatUser
			state.Mu.Unlock()

			supportJID := types.NewJID(getSupportNumber(), types.DefaultUserServer)

			if activeUserFull != "" {
				userJID, _ := types.ParseJID(activeUserFull)
				sendTextMessage(ctx, client, userJID, formatarMensagem(config.GetConfig().MsgSuporteAssumido, map[string]string{"agente": agent}))
				sendTextMessage(ctx, client, supportJID, fmt.Sprintf(
					"✅ Você assumiu o atendimento como *%s*.\n\nPara responder o usuário, comece a mensagem com (!).\nPara finalizar, digite *#encerrar*.",
					agent,
				))
			} else {
				sendTextMessage(ctx, client, supportJID, "⚠️ *Aviso do Sistema:* Não há nenhum atendimento pendente para assumir no momento.")
			}

			return true
		}
	}

	return false
}

// Função tratarVotoConfirmacaoIdentidade executa a regra de negócio/rotina correspondente
func tratarVotoConfirmacaoIdentidade(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, isSim, isNao bool) {
	if isSim {
		state.Mu.Lock()
		nomeCompleto := state.Names[sender]
		primeiroNome := strings.Split(nomeCompleto, " ")[0]
		state.Mu.Unlock()

		sendTextMessage(ctx, client, v.Info.Chat, fmt.Sprintf("Certo, %s! 😎", primeiroNome))
		time.Sleep(500 * time.Millisecond)
		SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
	} else if isNao {
		state.Mu.Lock()
		delete(state.Names, sender)
		delete(state.UserIDs, sender)
		uState.Step = 1
		state.Mu.Unlock()

		sendTextMessage(ctx, client, v.Info.Chat, "Sem problemas! Para começarmos, como você se chama? (Pode digitar seu nome completo)")
	}
}

// Função tratarVotoEnqueteDocumentos executa a regra de negócio/rotina correspondente
func tratarVotoEnqueteDocumentos(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, isSim, isNao bool, cfg config.Config) {
	if isSim {
		state.Mu.Lock()
		uState.Step = 41
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, cfg.MsgEnviarDocumentos)
	} else if isNao {
		node, found := FindNodeByID(uState.CurrentNodeID)
		askImages := !found || node.GetAskImages()
		if askImages {
			state.Mu.Lock()
			uState.Step = 35
			state.Mu.Unlock()
			pollMsg := client.BuildPollCreation(cfg.MsgEnqueteFotos, []string{"Sim", "Não"}, 1)
			_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
		} else {
			FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
		}
	}
}

// Função tratarVotoConfirmacaoDocumentos executa a regra de negócio/rotina correspondente
func tratarVotoConfirmacaoDocumentos(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, isSim, isNao bool, cfg config.Config) {
	if isSim {
		node, found := FindNodeByID(uState.CurrentNodeID)
		askImages := !found || node.GetAskImages()
		if askImages {
			state.Mu.Lock()
			uState.Step = 35
			state.Mu.Unlock()
			pollMsg := client.BuildPollCreation(cfg.MsgEnqueteFotosPosDocs, []string{"Sim", "Não"}, 1)
			_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)
		} else {
			FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
		}
	} else if isNao {
		state.Mu.Lock()
		uState.Step = 41
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, cfg.MsgProximoDocumento)
	}
}

// Função tratarVotoEnqueteFotos executa a regra de negócio/rotina correspondente
func tratarVotoEnqueteFotos(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, isSim, isNao bool, cfg config.Config) {
	if isSim {
		state.Mu.Lock()
		uState.Step = 36
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, cfg.MsgEnviarFotos)
	} else if isNao {
		FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
	}
}

// Função tratarVotoConfirmacaoFotos executa a regra de negócio/rotina correspondente
func tratarVotoConfirmacaoFotos(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, isSim, isNao bool, cfg config.Config) {
	if isSim {
		FinalizarChamadoEAlertar(ctx, client, v, uState, sender)
	} else if isNao {
		state.Mu.Lock()
		uState.Step = 36
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, cfg.MsgProximaFoto)
	}
}

// Função tratarVotoNovaMensagemChamado executa a regra de negócio/rotina correspondente
func tratarVotoNovaMensagemChamado(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, isSim, isNao bool) {
	if isSim {
		state.Mu.Lock()
		uState.Step = 52
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, "✍️ Por favor, digite a mensagem que deseja adicionar ao chamado:")
	} else if isNao {
		sendTextMessage(ctx, client, v.Info.Chat, "Tudo bem! Agradecemos o contato. O acompanhamento deste chamado foi encerrado. Quando precisar, é só mandar uma nova mensagem! 🚀")

		state.Mu.Lock()
		uState.Step = -1
		uState.ActiveTicketID = 0
		uState.LastGreetingTime = time.Now().Add(-1 * time.Minute)
		state.Mu.Unlock()
	}
}

// ─── Enquete de seleção de nome ───────────────────────────────────────────────

// tratarVotoNomeSistema processa a seleção do nome do usuário na lista do GLPI.

// Função tratarVotoNomeSistema executa a regra de negócio/rotina correspondente
func tratarVotoNomeSistema(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, selectedHash []byte) {
	state.Mu.Lock()
	opcoesSalvas := uState.PollOptions
	idsSalvos := uState.PollIDs
	state.Mu.Unlock()

	var opcaoSelecionada string
	var idSelecionado int

	for i, opcao := range opcoesSalvas {
		if matchOptionHash(selectedHash, opcao) {
			opcaoSelecionada = opcao
			if opcao != "Nenhum desses" && i < len(idsSalvos) {
				idSelecionado = idsSalvos[i]
			}
			break
		}
	}

	if opcaoSelecionada == "" {
		return
	}

	if opcaoSelecionada == "Nenhum desses" {
		state.Mu.Lock()
		uState.Step = 12
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, "Sem problemas! Por favor, digite o seu *Nome e Sobrenome* para registrarmos no chamado:")
		return
	}

	primeiroNome := strings.Split(opcaoSelecionada, " ")[0]

	state.Mu.Lock()
	state.Names[sender] = opcaoSelecionada
	state.UserIDs[sender] = idSelecionado
	state.Mu.Unlock()

	sendTextMessage(ctx, client, v.Info.Chat, fmt.Sprintf("Certo, %s! 😎", primeiroNome))
	time.Sleep(500 * time.Millisecond)

	SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
}

// ─── Menu raiz e fluxo dinâmico ───────────────────────────────────────────────

// SendRootFlowPoll envia a enquete do menu raiz (nível 0) do fluxo configurado.

// Função SendRootFlowPoll executa a regra de negócio/rotina correspondente
func SendRootFlowPoll(ctx context.Context, client *whatsmeow.Client, jid types.JID, uState *state.UserState) {
	root := GetFlowConfig()
	options := make([]string, 0, len(root.Children))
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

// tratarVotoFluxoDinamico processa o voto no menu de navegação do fluxo configurável.

// Função tratarVotoFluxoDinamico executa a regra de negócio/rotina correspondente
func tratarVotoFluxoDinamico(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, selectedHash []byte) {
	state.Mu.Lock()
	currentNodeID := uState.CurrentNodeID
	if currentNodeID == "" {
		currentNodeID = "root"
	}
	state.Mu.Unlock()

	fmt.Printf("ℹ️ [FLUXO] Iniciando tratarVotoFluxoDinamico para %s. currentNodeID: %q\n", sender, currentNodeID)

	// Botão "Voltar"
	if matchOptionHash(selectedHash, "⬅️ Voltar") {
		tratarNavegacaoVoltar(ctx, client, v, uState, sender, currentNodeID)
		return
	}

	currentNode, found := FindNodeByID(currentNodeID)
	if !found {
		fmt.Printf("🚨 [FLUXO] Nó corrente %q não encontrado para %s!\n", currentNodeID, sender)
		SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
		return
	}

	selectedChild, foundChild := encontrarFilhoSelecionado(currentNode, selectedHash, sender)
	if !foundChild {
		fmt.Printf("⚠️ [FLUXO] Nenhuma opção correspondente para o hash %x no nó %q para %s\n", selectedHash, currentNodeID, sender)
		return
	}

	fmt.Printf("✅ [FLUXO] Filho selecionado: ID=%s, Title=%q, Type=%s\n", selectedChild.ID, selectedChild.Title, selectedChild.Type)

	executarAcaoNodo(ctx, client, v, uState, sender, selectedChild)
}

// tratarNavegacaoVoltar volta ao nó pai na hierarquia do fluxo.

// Função tratarNavegacaoVoltar executa a regra de negócio/rotina correspondente
func tratarNavegacaoVoltar(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender, currentNodeID string) {
	fmt.Printf("ℹ️ [FLUXO] Usuário %s clicou em voltar a partir do nó %q\n", sender, currentNodeID)

	parent, found := FindParentNodeByID(currentNodeID)
	if !found || parent.ID == "" {
		SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
		return
	}

	options := buildChildOptions(parent)

	state.Mu.Lock()
	uState.CurrentNodeID = parent.ID
	uState.Step = 1000
	state.Mu.Unlock()

	_, _ = sendMessage(ctx, client, v.Info.Chat, buildMenuPoll(client, parent, options))
}

// encontrarFilhoSelecionado percorre os filhos do nó atual procurando a opção votada.

// Função encontrarFilhoSelecionado executa a regra de negócio/rotina correspondente
func encontrarFilhoSelecionado(currentNode FlowNode, selectedHash []byte, sender string) (FlowNode, bool) {
	fmt.Printf("ℹ️ [FLUXO] Nó %q encontrado. Procurando entre %d filhos...\n", currentNode.ID, len(currentNode.Children))

	for _, child := range currentNode.Children {
		matched := matchOptionHash(selectedHash, child.Title)
		fmt.Printf("   - Filho: ID=%s, Title=%q, Type=%s -> Matched: %t\n", child.ID, child.Title, child.Type, matched)
		if matched {
			return child, true
		}
	}

	return FlowNode{}, false
}

// executarAcaoNodo executa a ação correspondente ao tipo do nó selecionado.

// Função executarAcaoNodo executa a regra de negócio/rotina correspondente
func executarAcaoNodo(ctx context.Context, client *whatsmeow.Client, v *events.Message, uState *state.UserState, sender string, node FlowNode) {
	switch node.Type {
	case NodeMenu:
		if len(node.Children) == 0 {
			sendTextMessage(ctx, client, v.Info.Chat, "⚠️ Esta opção de menu está vazia ou em construção no momento.")
			time.Sleep(1 * time.Second)
			SendRootFlowPoll(ctx, client, v.Info.Chat, uState)
			return
		}

		options := buildChildOptions(node)
		if node.GetShowBackButton() {
			options = append(options, "⬅️ Voltar")
		}

		state.Mu.Lock()
		uState.CurrentNodeID = node.ID
		uState.Step = 1000
		state.Mu.Unlock()

		pollMsg := client.BuildPollCreation(fmt.Sprintf("Qual o problema com %s?", node.Title), options, 1)
		_, _ = sendMessage(ctx, client, v.Info.Chat, pollMsg)

	case NodeText:
		sendTextMessage(ctx, client, v.Info.Chat, node.Content)
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
		uState.CategoryID = node.GLPIID
		uState.SubCategory = node.Title
		uState.CurrentNodeID = node.ID
		uState.Step = 2
		state.Mu.Unlock()
		sendTextMessage(ctx, client, v.Info.Chat, fmt.Sprintf("Certo, problema *%s* anotado!\nAgora, por favor, digite um **título curto** para o problema:", node.Title))
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// parsearAtendentes converte a string de atendentes separada por vírgula em uma slice.

// Função parsearAtendentes executa a regra de negócio/rotina correspondente
func parsearAtendentes(agentsStr string) []string {
	var agents []string
	for _, p := range strings.Split(agentsStr, ",") {
		if p = strings.TrimSpace(p); p != "" {
			agents = append(agents, p)
		}
	}
	return agents
}
