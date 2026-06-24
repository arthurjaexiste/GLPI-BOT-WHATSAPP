package state

import (
	"sync"
	"time"
)

// Doc representa um arquivo/documento enviado pelo usuário via WhatsApp.
type Doc struct {
	Bytes []byte
	Name  string
}

// UserState mantém o estado da conversa de cada usuário durante o atendimento.
type UserState struct {
	Step             int
	Title            string
	Description      string
	InvalidAttempts  int
	LastTicketTime   time.Time
	LastGreetingTime time.Time
	PollOptions      []string
	PollIDs          []int
	CategoryID       int
	Images           [][]byte
	Docs             []Doc
	SubCategory      string
	ActiveTicketID   int
	CurrentNodeID    string
}

// ─── Estado global compartilhado ─────────────────────────────────────────────

var (
	Mu      sync.Mutex
	Users   = make(map[string]*UserState)
	Names   = make(map[string]string)
	UserIDs = make(map[string]int)

	// Controle da fila do Chat ao Vivo
	ActiveLiveChatUser string
	ActiveAgentName    string
	LiveChatQueue      []string
)