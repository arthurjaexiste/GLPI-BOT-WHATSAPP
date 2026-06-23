package state

import (
	"sync"
	"time"
)

type Doc struct {
	Bytes []byte
	Name  string
}

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

var (
	Users   = make(map[string]*UserState)
	Names   = make(map[string]string)
	UserIDs = make(map[string]int)
	Mu      sync.Mutex

	// Controle da Fila do Chat ao Vivo
	ActiveLiveChatUser string
	ActiveAgentName    string // É essa variável aqui que o Go estava sentindo falta!
	LiveChatQueue      []string
)