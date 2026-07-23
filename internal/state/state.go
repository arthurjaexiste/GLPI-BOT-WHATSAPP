// ============================================================================
// ARQUIVO: state.go
// Descrição: Implementação Go (backend) para o ecossistema GLPI-BOT.
// ============================================================================

package state

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Doc representa um arquivo/documento enviado pelo usuário via WhatsApp.
// Struct Doc define a estrutura de dados e mapeamento correspondente
type Doc struct {
	Bytes []byte
	Name  string
}

// UserState mantém o estado da conversa de cada usuário durante o atendimento.
// Struct UserState define a estrutura de dados e mapeamento correspondente
type UserState struct {
	Step                int
	Title               string
	Description         string
	InvalidAttempts     int
	LastTicketTime      time.Time
	LastGreetingTime    time.Time
	LastInteractionTime time.Time
	PollOptions         []string
	PollIDs             []int
	CategoryID          int
	Images              [][]byte
	Docs                []Doc
	SubCategory         string
	ActiveTicketID      int
	CurrentNodeID       string
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

// PersistedState encapsula os mapas globais para gravação física
type PersistedState struct {
	Users   map[string]*UserState `json:"users"`
	Names   map[string]string     `json:"names"`
	UserIDs map[string]int        `json:"user_ids"`
}

// SaveState grava as informações atuais de sessões de usuários em disco
func SaveState() {
	Mu.Lock()
	defer Mu.Unlock()

	data := PersistedState{
		Users:   Users,
		Names:   Names,
		UserIDs: UserIDs,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	_ = os.WriteFile("db/state_persist.json", jsonData, 0666)
}

// LoadState recupera as sessões salvas em disco na inicialização do bot
func LoadState() {
	Mu.Lock()
	defer Mu.Unlock()

	data, err := os.ReadFile("db/state_persist.json")
	if err != nil {
		return
	}

	var stateData PersistedState
	if err := json.Unmarshal(data, &stateData); err == nil {
		if stateData.Users != nil {
			Users = stateData.Users
		}
		if stateData.Names != nil {
			Names = stateData.Names
		}
		if stateData.UserIDs != nil {
			UserIDs = stateData.UserIDs
		}
	}
}

// StartPersister inicia um daemon em background para gravar alterações a cada 3 segundos
func StartPersister() {
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		for range ticker.C {
			SaveState()
		}
	}()
}