package whatsapp

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// NodeType descreve o comportamento de um nó no fluxo de conversa.
type NodeType string

const (
	NodeMenu         NodeType = "menu"   // Exibe um submenu de opções
	NodeText         NodeType = "text"   // Responde com texto/FAQ e encerra a interação
	NodeGLPITicket   NodeType = "ticket" // Inicia a abertura de um chamado no GLPI
	NodeGLPIStatus   NodeType = "status" // Consulta o status de um chamado existente
	NodeHumanSupport NodeType = "human"  // Encaminha para atendimento humano (Live Chat)
)

// FlowNode representa um nó do fluxo de conversa configurável pelo painel web.
type FlowNode struct {
	ID             string     `json:"id"`
	Title          string     `json:"title"`
	Type           NodeType   `json:"type"`
	Content        string     `json:"content,omitempty"`          // Texto (NodeText) ou prompt customizado (NodeGLPITicket)
	GLPIID         int        `json:"glpi_id,omitempty"`          // ID da categoria no GLPI
	Children       []FlowNode `json:"children,omitempty"`         // Filhos do menu
	AskImages      *bool      `json:"ask_images,omitempty"`       // Solicitar fotos/prints
	AskDocs        *bool      `json:"ask_docs,omitempty"`         // Solicitar documentos/arquivos
	ShowBackButton *bool      `json:"show_back_button,omitempty"` // Exibir opção "⬅️ Voltar"
}

// GetAskImages retorna true (padrão) se o nó deve solicitar imagens.
func (n FlowNode) GetAskImages() bool {
	if n.AskImages == nil {
		return true
	}
	return *n.AskImages
}

// GetAskDocs retorna true (padrão) se o nó deve solicitar documentos.
func (n FlowNode) GetAskDocs() bool {
	if n.AskDocs == nil {
		return true
	}
	return *n.AskDocs
}

// GetShowBackButton retorna true (padrão) se o nó deve exibir o botão de voltar.
func (n FlowNode) GetShowBackButton() bool {
	if n.ShowBackButton == nil {
		return true
	}
	return *n.ShowBackButton
}

// boolPtr é um helper para criar um *bool a partir de um valor literal.
func boolPtr(b bool) *bool { return &b }

// ─── Estado do fluxo ─────────────────────────────────────────────────────────

var (
	flowMutex sync.RWMutex
	rootNode  FlowNode
	flowPath  = "db/flow.json"
)

// ─── Inicialização ────────────────────────────────────────────────────────────

// InitFlow carrega o fluxo de conversa do disco. Cria o arquivo padrão se necessário.
func InitFlow() {
	if err := loadFlow(); err != nil {
		fmt.Println("⚠️ Erro ao carregar fluxo, criando arquivo padrão:", err)
		createDefaultFlowJSON()
		_ = loadFlow()
	} else {
		fmt.Println("✅ Fluxo de conversa dinâmico carregado com sucesso do JSON.")
	}
}

// ─── API pública ──────────────────────────────────────────────────────────────

// GetFlowConfig retorna uma cópia thread-safe do nó raiz do fluxo.
func GetFlowConfig() FlowNode {
	flowMutex.RLock()
	defer flowMutex.RUnlock()
	return rootNode
}

// SaveFlowConfig persiste um novo fluxo no disco e atualiza o estado em memória.
func SaveFlowConfig(root FlowNode) error {
	flowMutex.Lock()
	defer flowMutex.Unlock()

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(flowPath, data, 0644); err != nil {
		return err
	}

	rootNode = root
	return nil
}

// FindNodeByID busca um nó pelo ID de forma recursiva (thread-safe).
func FindNodeByID(id string) (FlowNode, bool) {
	flowMutex.RLock()
	defer flowMutex.RUnlock()
	return findNodeRecursive(rootNode, id)
}

// FindParentNodeByID busca o nó pai de um nó dado, permitindo a navegação "Voltar".
func FindParentNodeByID(childID string) (FlowNode, bool) {
	flowMutex.RLock()
	defer flowMutex.RUnlock()
	return findParentRecursive(rootNode, childID)
}

// ─── Helpers privados ─────────────────────────────────────────────────────────

func loadFlow() error {
	flowMutex.Lock()
	defer flowMutex.Unlock()

	_ = os.MkdirAll("db", 0777)

	data, err := os.ReadFile(flowPath)
	if err != nil {
		return err
	}

	var root FlowNode
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}

	rootNode = root
	return nil
}

func findNodeRecursive(node FlowNode, id string) (FlowNode, bool) {
	if node.ID == id {
		return node, true
	}
	for _, child := range node.Children {
		if found, ok := findNodeRecursive(child, id); ok {
			return found, true
		}
	}
	return FlowNode{}, false
}

func findParentRecursive(current FlowNode, childID string) (FlowNode, bool) {
	for _, child := range current.Children {
		if child.ID == childID {
			return current, true
		}
		if found, ok := findParentRecursive(child, childID); ok {
			return found, true
		}
	}
	return FlowNode{}, false
}

// createDefaultFlowJSON grava um fluxo padrão com as três opções básicas do bot.
func createDefaultFlowJSON() {
	defaultRoot := FlowNode{
		ID:             "root",
		Title:          "Menu Inicial",
		Type:           NodeMenu,
		ShowBackButton: boolPtr(true),
		Children: []FlowNode{
			{
				ID:        "abrir_chamado",
				Title:     "Abrir Chamado",
				Type:      NodeGLPITicket,
				GLPIID:    0,
				Content:   "Por favor, descreva o seu problema detalhadamente:",
				AskImages: boolPtr(true),
				AskDocs:   boolPtr(true),
			},
			{
				ID:    "acompanhar_chamado",
				Title: "Acompanhar Chamado",
				Type:  NodeGLPIStatus,
			},
			{
				ID:    "falar_suporte",
				Title: "Falar com o Suporte",
				Type:  NodeHumanSupport,
			},
		},
	}

	data, _ := json.MarshalIndent(defaultRoot, "", "  ")
	_ = os.WriteFile(flowPath, data, 0644)
}
