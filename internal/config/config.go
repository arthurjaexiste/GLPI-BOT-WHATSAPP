package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	GLPIApiURL    string `json:"glpi_api_url"`
	GLPIAppToken  string `json:"glpi_app_token"`
	GLPIUserToken string `json:"glpi_user_token"`

	CompanyName         string `json:"company_name"`
	TelefoneNotificacao string `json:"telefone_notificacao"`
	DarkList            string `json:"dark_list"`
	SupportAgents       string `json:"support_agents"`

	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPSender   string `json:"smtp_sender"`
	SMTPReceiver string `json:"smtp_receiver"`
	SMTPEnabled  bool   `json:"smtp_enabled"`

	WorkingHoursStart   string `json:"working_hours_start"`
	WorkingHoursEnd     string `json:"working_hours_end"`
	WorkingDays         string `json:"working_days"`
	WorkingHoursEnabled bool   `json:"working_hours_enabled"`
	MsgAusencia         string `json:"msg_ausencia"`

	MsgNovoUsuario                string `json:"msg_novo_usuario"`
	MsgUsuarioExistente           string `json:"msg_usuario_existente"`
	MsgMenuInicial                string `json:"msg_menu_inicial"`
	MsgTicketCriado               string `json:"msg_ticket_criado"`
	MsgFilaSuporte                string `json:"msg_fila_suporte"`
	MsgFilaEspera                 string `json:"msg_fila_espera"`
	MsgSuporteAssumido            string `json:"msg_suporte_assumido"`
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

func InitConfig() {
	mu.Lock()
	defer mu.Unlock()

	_ = os.MkdirAll("db", 0777)

	if data, err := os.ReadFile(configPath); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil {
			globalConfig = cfg
			preencherDefaultsMensagens(&globalConfig)
			_ = saveConfigLocked()
			return
		}
	}

	globalConfig = carregarConfigDoAmbiente()
	preencherDefaultsMensagens(&globalConfig)
	_ = saveConfigLocked()
}

func GetConfig() Config {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig
}

func UpdateConfig(newCfg Config) error {
	mu.Lock()
	defer mu.Unlock()

	preencherDefaultsMensagens(&newCfg)
	globalConfig = newCfg
	return saveConfigLocked()
}

func SaveConfig(cfg Config) error {
	return UpdateConfig(cfg)
}

func saveConfigLocked() error {
	data, err := json.MarshalIndent(globalConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func carregarConfigDoAmbiente() Config {
	env := parseEnvFile()

	get := func(key, defaultValue string) string {
		if val, ok := env[key]; ok && val != "" {
			return val
		}
		if val := os.Getenv(key); val != "" {
			return val
		}
		return defaultValue
	}

	smtpPortStr := get("SMTP_PORT", "587")
	smtpPort, _ := strconv.Atoi(smtpPortStr)
	smtpEnabled := strings.ToLower(get("SMTP_ENABLED", "false"))
	whEnabled := strings.ToLower(get("WORKING_HOURS_ENABLED", "false"))

	return Config{
		GLPIApiURL:    get("GLPI_API_URL", ""),
		GLPIAppToken:  get("GLPI_APP_TOKEN", ""),
		GLPIUserToken: get("GLPI_USER_TOKEN", ""),

		CompanyName:         get("COMPANY_NAME", "Suporte TI"),
		TelefoneNotificacao: get("TELEFONE_NOTIFICACAO", ""),
		DarkList:            get("DARK_LIST", ""),
		SupportAgents:       get("SUPPORT_AGENTS", ""),

		SMTPHost:     get("SMTP_HOST", ""),
		SMTPPort:     smtpPort,
		SMTPUsername: get("SMTP_USERNAME", ""),
		SMTPPassword: get("SMTP_PASSWORD", ""),
		SMTPSender:   get("SMTP_SENDER", ""),
		SMTPReceiver: get("SMTP_RECEIVER", ""),
		SMTPEnabled:  smtpEnabled == "true" || smtpEnabled == "1",

		WorkingHoursStart:   get("WORKING_HOURS_START", "08:00"),
		WorkingHoursEnd:     get("WORKING_HOURS_END", "18:00"),
		WorkingDays:         get("WORKING_DAYS", "1,2,3,4,5"),
		WorkingHoursEnabled: whEnabled == "true" || whEnabled == "1",
		MsgAusencia:         get("MSG_AUSENCIA", ""),
	}
}

func parseEnvFile() map[string]string {
	env := make(map[string]string)

	data, err := os.ReadFile(".env")
	if err != nil {
		return env
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if len(val) >= 2 {
			first, last := val[0], val[len(val)-1]
			if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		env[key] = val
	}

	return env
}

func preencherDefaultsMensagens(cfg *Config) {
	defaults := map[*string]string{
		&cfg.MsgNovoUsuario: "{saudacao}! Sou o bot de atendimento da {empresa}. 👋\n\n" +
			"💡 _Dica: Se escolher a opção errada, digite * a qualquer momento para voltar._\n\n" +
			"Para começarmos, como você se chama? (Pode digitar seu nome completo)",

		&cfg.MsgUsuarioExistente: "{saudacao}! Bem-vindo de volta ao suporte da {empresa}. 👋\n\n" +
			"💡 _Dica: Se escolher a opção errada, digite * a qualquer momento para voltar._",

		&cfg.MsgMenuInicial: "Como posso te ajudar hoje?",

		&cfg.MsgTicketCriado: "✅ Chamado *#{ticket_id}* criado com sucesso! Em breve um técnico entrará em contato.",

		&cfg.MsgFilaSuporte: "⏳ Solicitação enviada! Aguarde um momento até que um técnico aceite o seu atendimento.",

		&cfg.MsgFilaEspera: "⏳ Nossos técnicos estão em outro atendimento no momento.\n\n" +
			"Você entrou na fila de espera (Sua posição: *{posicao}*).\n" +
			"Assim que o técnico liberar, você será conectado automaticamente.\n\n" +
			"Para sair da fila, digite *#cancelar*.",

		&cfg.MsgSuporteAssumido: "✅ O técnico *{agente}* assumiu o seu atendimento!\n\n" +
			"Tudo que você digitar agora será enviado a ele.\nPara finalizar, digite *#encerrar*.",

		&cfg.MsgFimAtendimento: "✅ Atendimento ao vivo encerrado.\n\n" +
			"Agradecemos o contato! Quando precisar de algo, é só mandar uma nova mensagem. 🚀",

		&cfg.MsgEnqueteDocumentos:          "Você possui arquivos ou documentos (PDF, Word, Excel, etc) para enviar?",
		&cfg.MsgEnviarDocumentos:           "Pode enviar seus arquivos ou documentos! 📄",
		&cfg.MsgEnqueteConfirmarDocumentos: "📄 Arquivo recebido! Já terminou de enviar seus documentos?",
		&cfg.MsgDocumentoAdicionado:        "✅ Mais um documento adicionado à lista!",
		&cfg.MsgProximoDocumento:           "📄 Beleza, pode enviar o próximo arquivo!",
		&cfg.MsgEnqueteFotos:               "Você tem alguma FOTO ou PRINT do problema para enviar?",
		&cfg.MsgEnqueteFotosPosDocs:        "Arquivos salvos! Você tem alguma FOTO ou PRINT do problema para enviar?",
		&cfg.MsgEnviarFotos:                "Pode enviar suas fotos! 📸",
		&cfg.MsgEnqueteConfirmarFotos:      "📸 Foto recebida! Já terminou de enviar suas fotos?",
		&cfg.MsgFotoAdicionada:             "✅ Mais uma foto adicionada à lista!",
		&cfg.MsgProximaFoto:                "📸 Beleza, pode enviar a próxima foto!",

		&cfg.MsgAusencia: "Anotamos seu problema! 📝 No momento estamos fora do horário de expediente. " +
			"Nosso atendimento retorna no próximo dia útil às {inicio}h.",
	}

	for field, defaultValue := range defaults {
		if *field == "" {
			*field = defaultValue
		}
	}
}
