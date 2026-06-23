package config

import (
	"encoding/json"
	"os"
	"strconv"
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

	SMTPHost            string `json:"smtp_host"`
	SMTPPort            int    `json:"smtp_port"`
	SMTPUsername        string `json:"smtp_username"`
	SMTPPassword        string `json:"smtp_password"`
	SMTPSender          string `json:"smtp_sender"`
	SMTPReceiver        string `json:"smtp_receiver"`
	SMTPEnabled         bool   `json:"smtp_enabled"`
	MsgNovoUsuario      string `json:"msg_novo_usuario"`
	MsgUsuarioExistente string `json:"msg_usuario_existente"`
	MsgMenuInicial      string `json:"msg_menu_inicial"`
	MsgTicketCriado     string `json:"msg_ticket_criado"`
	MsgFilaSuporte      string `json:"msg_fila_suporte"`
	MsgFilaEspera       string `json:"msg_fila_espera"`
	MsgSuporteAssumido  string `json:"msg_suporte_assumido"`
	MsgFimAtendimento             string `json:"msg_fim_atendimento"`
	MsgEnqueteDocumentos          string `json:"msg_enquete_documentos"`
	MsgEnviarDocumentos           string `json:"msg_enviar_documentos"`
	MsgEnqueteConfirmarDocumentos string `json:"msg_enquete_confirmar_documentos"`
	MsgDocumentoAdicionado        string `json:"msg_documento_adicionado"`
	MsgProximoDocumento           string `json:"msg_proximo_documento"`
	MsgEnqueteFotos               string `json:"msg_enquete_fotos"`
	MsgEnqueteFotosPosDocs        string `json:"msg_enquete_fotos_pos_docs"`
	MsgEnviarFotos                string `json:"msg_enviar_fotos"`
	MsgEnqueteConfirmarFotos      string `json:"msg_enquete_confirmar_fotos"`
	MsgFotoAdicionada             string `json:"msg_foto_adicionada"`
	MsgProximaFoto                string `json:"msg_proxima_foto"`
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
	if cfg.MsgEnqueteDocumentos == "" {
		cfg.MsgEnqueteDocumentos = "Você possui arquivos ou documentos (PDF, Word, Excel, etc) para enviar?"
	}
	if cfg.MsgEnviarDocumentos == "" {
		cfg.MsgEnviarDocumentos = "Pode enviar seus arquivos ou documentos! 📄"
	}
	if cfg.MsgEnqueteConfirmarDocumentos == "" {
		cfg.MsgEnqueteConfirmarDocumentos = "📄 Arquivo recebido! Já terminou de enviar seus documentos?"
	}
	if cfg.MsgDocumentoAdicionado == "" {
		cfg.MsgDocumentoAdicionado = "✅ Mais um documento adicionado à lista!"
	}
	if cfg.MsgProximoDocumento == "" {
		cfg.MsgProximoDocumento = "📄 Beleza, pode enviar o próximo arquivo!"
	}
	if cfg.MsgEnqueteFotos == "" {
		cfg.MsgEnqueteFotos = "Você tem alguma FOTO ou PRINT do problema para enviar?"
	}
	if cfg.MsgEnqueteFotosPosDocs == "" {
		cfg.MsgEnqueteFotosPosDocs = "Arquivos salvos! Você tem alguma FOTO ou PRINT do problema para enviar?"
	}
	if cfg.MsgEnviarFotos == "" {
		cfg.MsgEnviarFotos = "Pode enviar suas fotos! 📸"
	}
	if cfg.MsgEnqueteConfirmarFotos == "" {
		cfg.MsgEnqueteConfirmarFotos = "📸 Foto recebida! Já terminou de enviar suas fotos?"
	}
	if cfg.MsgFotoAdicionada == "" {
		cfg.MsgFotoAdicionada = "✅ Mais uma foto adicionada à lista!"
	}
	if cfg.MsgProximaFoto == "" {
		cfg.MsgProximaFoto = "📸 Beleza, pode enviar a próxima foto!"
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

	portVal := getVal("SMTP_PORT", "587")
	port, _ := strconv.Atoi(portVal)
	if port == 0 {
		port = 587
	}
	enabledVal := getVal("SMTP_ENABLED", "false")
	enabled := enabledVal == "true" || enabledVal == "1"

	globalConfig = Config{
		GLPIApiURL:          getVal("GLPI_API_URL", ""),
		GLPIAppToken:        getVal("GLPI_APP_TOKEN", ""),
		GLPIUserToken:       getVal("GLPI_USER_TOKEN", ""),
		CompanyName:         getVal("COMPANY_NAME", ""),
		TelefoneNotificacao: getVal("TELEFONE_NOTIFICACAO", ""),
		DarkList:            getVal("DARK_LIST", ""),
		SupportAgents:       getVal("SUPPORT_AGENTS", ""),

		SMTPHost:            getVal("SMTP_HOST", ""),
		SMTPPort:            port,
		SMTPUsername:        getVal("SMTP_USERNAME", ""),
		SMTPPassword:        getVal("SMTP_PASSWORD", ""),
		SMTPSender:          getVal("SMTP_SENDER", ""),
		SMTPReceiver:        getVal("SMTP_RECEIVER", ""),
		SMTPEnabled:         enabled,
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
