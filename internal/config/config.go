package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Config reúne todas as configurações do bot, persistidas em db/config.json.
type Config struct {
	// Integração GLPI
	GLPIApiURL    string `json:"glpi_api_url"`
	GLPIAppToken  string `json:"glpi_app_token"`
	GLPIUserToken string `json:"glpi_user_token"`

	// Identificação
	CompanyName         string `json:"company_name"`
	TelefoneNotificacao string `json:"telefone_notificacao"`
	DarkList            string `json:"dark_list"`
	SupportAgents       string `json:"support_agents"`

	// SMTP (alertas de desconexão)
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPSender   string `json:"smtp_sender"`
	SMTPReceiver string `json:"smtp_receiver"`
	SMTPEnabled  bool   `json:"smtp_enabled"`

	// Horário de atendimento
	WorkingHoursStart   string `json:"working_hours_start"`
	WorkingHoursEnd     string `json:"working_hours_end"`
	WorkingDays         string `json:"working_days"`
	WorkingHoursEnabled bool   `json:"working_hours_enabled"`
	MsgAusencia         string `json:"msg_ausencia"`

	// Mensagens configuráveis
	MsgNovoUsuario               string `json:"msg_novo_usuario"`
	MsgUsuarioExistente          string `json:"msg_usuario_existente"`
	MsgMenuInicial               string `json:"msg_menu_inicial"`
	MsgTicketCriado              string `json:"msg_ticket_criado"`
	MsgFilaSuporte               string `json:"msg_fila_suporte"`
	MsgFilaEspera                string `json:"msg_fila_espera"`
	MsgSuporteAssumido           string `json:"msg_suporte_assumido"`
	MsgFimAtendimento            string `json:"msg_fim_atendimento"`
	MsgEnqueteDocumentos         string `json:"msg_enquete_documentos"`
	MsgEnviarDocumentos          string `json:"msg_enviar_documentos"`
	MsgEnqueteConfirmarDocumentos string `json:"msg_enquete_confirmar_documentos"`
	MsgDocumentoAdicionado       string `json:"msg_documento_adicionado"`
	MsgProximoDocumento          string `json:"msg_proximo_documento"`
	MsgEnqueteFotos              string `json:"msg_enquete_fotos"`
	MsgEnqueteFotosPosDocs       string `json:"msg_enquete_fotos_pos_docs"`
	MsgEnviarFotos               string `json:"msg_enviar_fotos"`
	MsgEnqueteConfirmarFotos     string `json:"msg_enquete_confirmar_fotos"`
	MsgFotoAdicionada            string `json:"msg_foto_adicionada"`
	MsgProximaFoto               string `json:"msg_proxima_foto"`
}

var (
	globalConfig Config
	mu           sync.RWMutex
	configPath   = "db/config.json"
)

// ─── Inicialização ────────────────────────────────────────────────────────────

// InitConfig carrega a configuração do disco. Se o arquivo não existir ou for
// inválido, faz a migração automática a partir de variáveis de ambiente / .env.
func InitConfig() {
	mu.Lock()
	defer mu.Unlock()

	_ = os.MkdirAll("db", 0777)

	// Tenta carregar o config.json já existente
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg Config
		if err := json.Unmarshal(data, &cfg); err == nil {
			globalConfig = cfg
			preencherDefaultsMensagens(&globalConfig)
			_ = saveConfigLocked()
			return
		}
	}

	// Fallback: migração a partir de variáveis de ambiente / arquivo .env
	globalConfig = carregarConfigDoAmbiente()
	preencherDefaultsMensagens(&globalConfig)
	_ = saveConfigLocked()
}

// ─── API pública ──────────────────────────────────────────────────────────────

// GetConfig retorna uma cópia thread-safe da configuração atual.
func GetConfig() Config {
	mu.RLock()
	defer mu.RUnlock()
	return globalConfig
}

// SaveConfig persiste uma nova configuração e atualiza os valores em memória.
func SaveConfig(cfg Config) error {
	mu.Lock()
	defer mu.Unlock()
	globalConfig = cfg
	preencherDefaultsMensagens(&globalConfig)
	return saveConfigLocked()
}

// ─── Persistência ─────────────────────────────────────────────────────────────

// saveConfigLocked grava globalConfig em disco. Deve ser chamada com mu travado.
func saveConfigLocked() error {
	data, err := json.MarshalIndent(globalConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0666)
}

// ─── Migração do ambiente ─────────────────────────────────────────────────────

// carregarConfigDoAmbiente constrói uma Config a partir de variáveis de ambiente
// e de um arquivo .env opcional (usado apenas na primeira execução).
func carregarConfigDoAmbiente() Config {
	envMap := parseEnvFile()

	get := func(key, fallback string) string {
		if val, ok := envMap[key]; ok && val != "" {
			return val
		}
		if val := os.Getenv(key); val != "" {
			return val
		}
		return fallback
	}

	port, _ := strconv.Atoi(get("SMTP_PORT", "587"))
	if port == 0 {
		port = 587
	}

	smtpEnabled := get("SMTP_ENABLED", "false")
	whEnabled   := get("WORKING_HOURS_ENABLED", "false")

	return Config{
		GLPIApiURL:    get("GLPI_API_URL", ""),
		GLPIAppToken:  get("GLPI_APP_TOKEN", ""),
		GLPIUserToken: get("GLPI_USER_TOKEN", ""),

		CompanyName:         get("COMPANY_NAME", ""),
		TelefoneNotificacao: get("TELEFONE_NOTIFICACAO", ""),
		DarkList:            get("DARK_LIST", ""),
		SupportAgents:       get("SUPPORT_AGENTS", ""),

		SMTPHost:     get("SMTP_HOST", ""),
		SMTPPort:     port,
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

// parseEnvFile lê manualmente um arquivo .env, se existir, retornando suas
// chaves e valores. Usado apenas na migração inicial.
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

		// Remove aspas simples ou duplas ao redor do valor
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

// ─── Defaults de mensagens ────────────────────────────────────────────────────

// preencherDefaultsMensagens garante que nenhum campo de mensagem fique vazio,
// aplicando textos padrão caso o usuário não tenha configurado.
func preencherDefaultsMensagens(cfg *Config) {
	defaults := map[*string]string{
		&cfg.MsgNovoUsuario: "{saudacao}! Sou o bot de chamados da {empresa} 😎\n\n" +
			"💡 _Dica: Se escolher a opção errada, digite * a qualquer momento para voltar._\n\n" +
			"Para começarmos, como você se chama? (Pode digitar seu nome completo)",

		&cfg.MsgUsuarioExistente: "{saudacao}! Bem-vindo de volta ao suporte da {empresa}.\n\n" +
			"💡 _Dica: Se escolher a opção errada, digite * a qualquer momento para voltar._",

		&cfg.MsgMenuInicial: "Como posso te ajudar hoje?",

		&cfg.MsgTicketCriado: "✅ Chamado *#{ticket_id}* criado com sucesso! Em breve um técnico entrará em contato.",

		&cfg.MsgFilaSuporte: "⏳ Solicitação enviada! Aguarde um momento até que um técnico aceite o seu atendimento.",

		&cfg.MsgFilaEspera: "⏳ Nossos técnicos estão em outro atendimento no momento.\n\n" +
			"Você entrou na fila de espera (Sua posição: {posicao}).\n" +
			"Assim que o técnico liberar, você será conectado automaticamente.\n\n" +
			"Para sair da fila, digite *#cancelar*.",

		&cfg.MsgSuporteAssumido: "✅ O técnico *{agente}* assumiu o seu atendimento!\n\n" +
			"Tudo que você digitar agora será enviado a ele.\nPara finalizar, digite *#encerrar*.",

		&cfg.MsgFimAtendimento: "✅ Atendimento ao vivo encerrado.\n\n" +
			"Agradecemos o contato! Quando precisar de algo, é só mandar uma nova mensagem. 🚀",

		&cfg.MsgEnqueteDocumentos:          "Você possui arquivos ou documentos (PDF, Word, Excel, etc) para enviar?",
		&cfg.MsgEnviarDocumentos:            "Pode enviar seus arquivos ou documentos! 📄",
		&cfg.MsgEnqueteConfirmarDocumentos:  "📄 Arquivo recebido! Já terminou de enviar seus documentos?",
		&cfg.MsgDocumentoAdicionado:         "✅ Mais um documento adicionado à lista!",
		&cfg.MsgProximoDocumento:            "📄 Beleza, pode enviar o próximo arquivo!",
		&cfg.MsgEnqueteFotos:               "Você tem alguma FOTO ou PRINT do problema para enviar?",
		&cfg.MsgEnqueteFotosPosDocs:        "Arquivos salvos! Você tem alguma FOTO ou PRINT do problema para enviar?",
		&cfg.MsgEnviarFotos:               "Pode enviar suas fotos! 📸",
		&cfg.MsgEnqueteConfirmarFotos:     "📸 Foto recebida! Já terminou de enviar suas fotos?",
		&cfg.MsgFotoAdicionada:            "✅ Mais uma foto adicionada à lista!",
		&cfg.MsgProximaFoto:              "📸 Beleza, pode enviar a próxima foto!",

		&cfg.MsgAusencia: "Anotamos seu problema! 📝 No momento estamos fora do horário de expediente. " +
			"Nosso atendimento retorna no próximo dia útil às {inicio}h.",
	}

	for field, defaultValue := range defaults {
		if *field == "" {
			*field = defaultValue
		}
	}
}
