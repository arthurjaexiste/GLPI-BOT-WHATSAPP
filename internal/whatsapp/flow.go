// ============================================================================
// ARQUIVO: flow.go
// Descrição: Implementação Go (backend) para o ecossistema GLPI-BOT.
// ============================================================================

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
	Content        string     `json:"content,omitempty"`
	GLPIID         int        `json:"glpi_id,omitempty"`
	Children       []FlowNode `json:"children,omitempty"`
	AskImages      *bool      `json:"ask_images,omitempty"`
	AskDocs        *bool      `json:"ask_docs,omitempty"`
	ShowBackButton *bool      `json:"show_back_button,omitempty"`
}

func (n FlowNode) GetAskImages() bool {
	if n.AskImages == nil {
		return true
	}
	return *n.AskImages
}

func (n FlowNode) GetAskDocs() bool {
	if n.AskDocs == nil {
		return true
	}
	return *n.AskDocs
}

func (n FlowNode) GetShowBackButton() bool {
	if n.ShowBackButton == nil {
		return true
	}
	return *n.ShowBackButton
}

func boolPtr(b bool) *bool { return &b }

// ─── Estado do fluxo ─────────────────────────────────────────────────────────

var (
	flowMutex sync.RWMutex
	rootNode  FlowNode
	flowPath  = "db/flow.json"
)

// ─── Inicialização ────────────────────────────────────────────────────────────

func InitFlow() {
	flowMutex.Lock()
	defer flowMutex.Unlock()

	_ = os.MkdirAll("db", 0777)

	if data, err := os.ReadFile(flowPath); err == nil {
		var node FlowNode
		if err := json.Unmarshal(data, &node); err == nil && node.ID != "" {
			rootNode = node
			return
		}
	}

	createDefaultFlowJSON()
	if data, err := os.ReadFile(flowPath); err == nil {
		_ = json.Unmarshal(data, &rootNode)
	}
}

// ─── API pública ──────────────────────────────────────────────────────────────

func GetFlowTree() FlowNode {
	flowMutex.RLock()
	defer flowMutex.RUnlock()
	return rootNode
}

func GetFlowConfig() FlowNode {
	return GetFlowTree()
}

func UpdateFlowTree(newRoot FlowNode) error {
	flowMutex.Lock()
	defer flowMutex.Unlock()

	data, err := json.MarshalIndent(newRoot, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao converter fluxo para JSON: %w", err)
	}

	if err := os.WriteFile(flowPath, data, 0644); err != nil {
		return fmt.Errorf("erro ao salvar fluxo no disco: %w", err)
	}

	rootNode = newRoot
	return nil
}

func SaveFlowConfig(root FlowNode) error {
	return UpdateFlowTree(root)
}

func FindNodeByID(id string) (FlowNode, bool) {
	flowMutex.RLock()
	defer flowMutex.RUnlock()
	return findNodeRecursive(rootNode, id)
}

func FindParentNodeByID(childID string) (FlowNode, bool) {
	flowMutex.RLock()
	defer flowMutex.RUnlock()
	return findParentRecursive(rootNode, childID)
}

// ─── Helpers privados ─────────────────────────────────────────────────────────

func findNodeRecursive(current FlowNode, targetID string) (FlowNode, bool) {
	if current.ID == targetID {
		return current, true
	}
	for _, child := range current.Children {
		if found, ok := findNodeRecursive(child, targetID); ok {
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
