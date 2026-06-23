package whatsapp

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type NodeType string

const (
	NodeMenu         NodeType = "menu"   // Exibe opções/submenus
	NodeText         NodeType = "text"   // Responde com texto/FAQ e finaliza
	NodeGLPITicket   NodeType = "ticket" // Abre chamado no GLPI
	NodeGLPIStatus   NodeType = "status" // Acompanha chamado no GLPI
	NodeHumanSupport NodeType = "human"  // Falar com suporte (Live Chat)
)

type FlowNode struct {
	ID       string     `json:"id"`
	Title    string     `json:"title"`
	Type     NodeType   `json:"type"`
	Content  string     `json:"content,omitempty"`  // Mensagem de texto (se NodeText) ou prompt customizado (se NodeGLPITicket)
	GLPIID   int        `json:"glpi_id,omitempty"`  // ID da categoria no GLPI
	Children []FlowNode `json:"children,omitempty"` // Filhos (se NodeMenu)
}

var (
	flowMutex sync.RWMutex
	rootNode  FlowNode
	flowPath  = "db/flow.json"
)

func InitFlow() {
	err := loadFlow()
	if err != nil {
		fmt.Println("⚠️ Erro ao carregar fluxo, criando arquivo padrão:", err)
		createDefaultFlowJSON()
		_ = loadFlow()
	} else {
		fmt.Println("✅ Fluxo de conversa dinâmico carregado com sucesso do JSON.")
	}
}

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

func GetFlowConfig() FlowNode {
	flowMutex.RLock()
	defer flowMutex.RUnlock()
	return rootNode
}

func SaveFlowConfig(root FlowNode) error {
	flowMutex.Lock()
	defer flowMutex.Unlock()

	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}

	err = os.WriteFile(flowPath, data, 0644)
	if err != nil {
		return err
	}

	rootNode = root
	return nil
}

// FindNodeByID busca um nó pelo ID de forma recursiva
func FindNodeByID(id string) (FlowNode, bool) {
	flowMutex.RLock()
	defer flowMutex.RUnlock()
	return findNodeRecursive(rootNode, id)
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

// FindParentNodeByID busca o pai de um nó para permitir retroceder ("voltar")
func FindParentNodeByID(childID string) (FlowNode, bool) {
	flowMutex.RLock()
	defer flowMutex.RUnlock()
	return findParentRecursive(rootNode, childID)
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
		ID:    "root",
		Title: "Menu Inicial",
		Type:  NodeMenu,
		Children: []FlowNode{
			{
				ID:      "abrir_chamado",
				Title:   "Abrir Chamado",
				Type:    NodeGLPITicket,
				GLPIID:  0,
				Content: "Por favor, descreva o seu problema detalhadamente:",
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
