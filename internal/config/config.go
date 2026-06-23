package config

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

type Config struct {
	GLPIApiURL          string `json:"glpi_api_url"`
	GLPIAppToken        string `json:"glpi_app_token"`
	GLPIUserToken       string `json:"glpi_user_token"`
	CompanyName         string `json:"company_name"`
	TelefoneNotificacao string `json:"telefone_notificacao"`
	DarkList            string `json:"dark_list"`
	SupportAgents       string `json:"support_agents"`
	MsgNovoUsuario      string `json:"msg_novo_usuario"`
	MsgUsuarioExistente string `json:"msg_usuario_existente"`
	MsgMenuInicial      string `json:"msg_menu_inicial"`
	MsgTicketCriado     string `json:"msg_ticket_criado"`
	MsgFilaSuporte      string `json:"msg_fila_suporte"`
	MsgFilaEspera       string `json:"msg_fila_espera"`
	MsgSuporteAssumido  string `json:"msg_suporte_assumido"`
	MsgFimAtendimento   string `json:"msg_fim_atendimento"`
}

var (
	globalConfig Config
	mu           sync.RWMutex
	configPath   = "db/config.json"
)

// parseEnvFile lê manualmente o .env se existir para migração inicial
func parseEnvFile() map[string]string {
	env := make(map[string]string)
	data, err := os.ReadFile(".env")
	if err != nil {
		return env
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			env[key] = value
		}
	}
	return env
}

func preencherDefaultsMensagens(cfg *Config) {
	if cfg.MsgNovoUsuario == "" {
		cfg.MsgNovoUsuario = "{saudacao}! Sou o bot de chamados da {empresa} 😎\n\n💡 _Dica: Se escolher a opção errada, digite * a qualquer momento para voltar._\n\nPara começarmos, como você se chama? (Pode digitar seu nome completo)"
	}
	if cfg.MsgUsuarioExistente == "" {
		cfg.MsgUsuarioExistente = "{saudacao}! Bem-vindo de volta ao suporte da {empresa}.\n\n💡 _Dica: Se escolher a opção errada, digite * a qualquer momento para voltar._"
	}
	if cfg.MsgMenuInicial == "" {
		cfg.MsgMenuInicial = "Como posso te ajudar hoje?"
	}
	if cfg.MsgTicketCriado == "" {
		cfg.MsgTicketCriado = "✅ Chamado *#{ticket_id}* criado com sucesso! Em breve um técnico entrará em contato."
	}
	if cfg.MsgFilaSuporte == "" {
		cfg.MsgFilaSuporte = "⏳ Solicitação enviada! Aguarde um momento até que um técnico aceite o seu atendimento."
	}
	if cfg.MsgFilaEspera == "" {
		cfg.MsgFilaEspera = "⏳ Nossos técnicos estão em outro atendimento no momento.\n\nVocê entrou na fila de espera (Sua posição: {posicao}).\nAssim que o técnico liberar, você será conectado automaticamente.\n\nPara sair da fila, digite *#cancelar*."
	}
	if cfg.MsgSuporteAssumido == "" {
		cfg.MsgSuporteAssumido = "✅ O técnico *{agente}* assumiu o seu atendimento!\n\nTudo que você digitar agora será enviado a ele.\nPara finalizar, digite *#encerrar*."
	}
	if cfg.MsgFimAtendimento == "" {
		cfg.MsgFimAtendimento = "✅ Atendimento ao vivo encerrado.\n\nAgradecemos o contato! Quando precisar de algo, é só mandar uma nova mensagem. 🚀"
	}
}

func InitConfig() {
	mu.Lock()
	defer mu.Unlock()

	_ = os.MkdirAll("db", 0777)

	// 1. Tenta carregar do config.json
	data, err := os.ReadFile(configPath)
	if err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil {
			globalConfig = cfg
			preencherDefaultsMensagens(&globalConfig)
			_ = saveConfigLocked()
			return
		}
	}

	// 2. Se falhar ou não existir, faz a migração do .env e variáveis de ambiente
	envMap := parseEnvFile()
	
	getVal := func(key, fallback string) string {
		if val, exists := envMap[key]; exists && val != "" {
			return val
		}
		if val := os.Getenv(key); val != "" {
			return val
		}
		return fallback
	}

	globalConfig = Config{
		GLPIApiURL:          getVal("GLPI_API_URL", ""),
		GLPIAppToken:        getVal("GLPI_APP_TOKEN", ""),
		GLPIUserToken:       getVal("GLPI_USER_TOKEN", ""),
		CompanyName:         getVal("COMPANY_NAME", ""),
		TelefoneNotificacao: getVal("TELEFONE_NOTIFICACAO", ""),
		DarkList:            getVal("DARK_LIST", ""),
		SupportAgents:       getVal("SUPPORT_AGENTS", ""),
	}
	preencherDefaultsMensagens(&globalConfig)

	// Salva o config.json migrado
	_ = saveConfigLocked()
}

func GetConfig() Config {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig
}

func SaveConfig(cfg Config) error {
	mu.Lock()
	defer mu.Unlock()
	globalConfig = cfg
	preencherDefaultsMensagens(&globalConfig)
	return saveConfigLocked()
}

func saveConfigLocked() error {
	data, err := json.MarshalIndent(globalConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0666)
}
