// ============================================================================
// ARQUIVO: glpi.go
// Descrição: Implementação Go (backend) para o ecossistema GLPI-BOT.
// ============================================================================

package glpi

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bot-glpi/internal/config"
)

// httpClient é o cliente HTTP compartilhado com suporte a TLS/HTTPS interno e timeout curto (3s)
var httpClient = &http.Client{
	Timeout: 3 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

// getBaseURL retorna a URL base da API do GLPI formatada e sem barra final
func getBaseURL() string {
	rawURL := strings.TrimSpace(config.GetConfig().GLPIApiURL)
	rawURL = strings.TrimSuffix(rawURL, "/")
	if rawURL == "" {
		return ""
	}
	if strings.Contains(rawURL, ".php") || strings.Contains(rawURL, "apirest") || strings.HasSuffix(rawURL, "/v1") {
		return rawURL
	}
	return rawURL + "/apirest.php"
}

// setCommonHeaders adiciona os cabeçalhos obrigatórios para todas as requisições GLPI.
func setCommonHeaders(req *http.Request, appToken, sessionToken string) {
	req.Header.Set("App-Token", strings.TrimSpace(appToken))
	req.Header.Set("Content-Type", "application/json")
	if sessionToken != "" {
		req.Header.Set("Session-Token", strings.TrimSpace(sessionToken))
	}
}

// ─── Sessão ───────────────────────────────────────────────────────────────────

// GetGLPISession inicia uma sessão na API do GLPI testando a URL com suporte a fallback rápido
func GetGLPISession() (string, error) {
	cfg := config.GetConfig()
	rawURL := strings.TrimSpace(cfg.GLPIApiURL)
	appToken := strings.TrimSpace(cfg.GLPIAppToken)
	userTokenRaw := strings.TrimSpace(cfg.GLPIUserToken)

	if rawURL == "" || appToken == "" || userTokenRaw == "" {
		return "", fmt.Errorf("Configuração incompleta: URL, App-Token e User-Token da API do GLPI devem estar preenchidos em 'Conexão GLPI API'")
	}

	userToken := userTokenRaw
	if !strings.HasPrefix(strings.ToLower(userToken), "user_token ") {
		userToken = "user_token " + userToken
	}

	cand := strings.TrimSuffix(rawURL, "/")
	fmt.Printf("🌐 [GLPI API] Conectando em: %s/initSession ...\n", cand)
	token, err := tryGLPIInitSession(cand, appToken, userToken)
	if err == nil && token != "" {
		fmt.Println("✅ [GLPI API] Sessão iniciada com sucesso!")
		return token, nil
	}

	fmt.Printf("⚠️ [GLPI API] Tentativa em %s falhou (%v). Testando fallback /apirest.php...\n", cand, err)
	if !strings.Contains(cand, "apirest.php") {
		candFallback := cand + "/apirest.php"
		tokenFB, errFB := tryGLPIInitSession(candFallback, appToken, userToken)
		if errFB == nil && tokenFB != "" {
			fmt.Println("✅ [GLPI API] Sessão iniciada via fallback apirest.php!")
			return tokenFB, nil
		}
		return "", fmt.Errorf("Falha de conexão no GLPI (%s: %v)", cand, err)
	}

	return "", fmt.Errorf("Falha de conexão no GLPI (%s: %v)", cand, err)
}

func tryGLPIInitSession(apiURL, appToken, userToken string) (string, error) {
	urlStr := fmt.Sprintf("%s/initSession", strings.TrimSuffix(apiURL, "/"))
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("App-Token", appToken)
	req.Header.Set("Authorization", userToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	token, ok := result["session_token"].(string)
	if !ok {
		return "", fmt.Errorf("session_token ausente na resposta")
	}

	return token, nil
}

// ─── Chamados ─────────────────────────────────────────────────────────────────

// CriarChamado abre um novo ticket no GLPI e retorna o ID gerado.

// Função CriarChamado executa a regra de negócio/rotina correspondente
func CriarChamado(sessionToken, titulo, descricao string, urgencia, requesterID, categoryID int) (int, error) {
	cfg := config.GetConfig()
	urlStr := fmt.Sprintf("%s/Ticket", getBaseURL())

	input := map[string]interface{}{
		"name":     titulo,
		"content":  descricao,
		"priority": urgencia,
	}
	if requesterID > 0 {
		input["_users_id_requester"] = requesterID
	}
	if categoryID > 0 {
		input["itilcategories_id"] = categoryID
	}

	payload := map[string]interface{}{"input": input}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	setCommonHeaders(req, cfg.GLPIAppToken, sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("erro ao criar chamado (HTTP %d): %s", resp.StatusCode, body)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if idFloat, ok := result["id"].(float64); ok {
		return int(idFloat), nil
	}

	return 0, fmt.Errorf("id do chamado não retornado")
}

// BuscarChamado retorna status (string), código numérico do status e título de um ticket.

// Função BuscarChamado executa a regra de negócio/rotina correspondente
func BuscarChamado(sessionToken string, ticketID int) (string, int, string, error) {
	cfg := config.GetConfig()
	urlStr := fmt.Sprintf("%s/Ticket/%d", getBaseURL(), ticketID)

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return "", 0, "", err
	}
	setCommonHeaders(req, cfg.GLPIAppToken, sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, "", fmt.Errorf("chamado não encontrado ou acesso negado")
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", 0, "", err
	}

	statusInt := extrairStatusInt(result["status"])
	titulo, _ := result["name"].(string)

	statusStr := mapaStatusGLPI[statusInt]
	if statusStr == "" {
		statusStr = "Desconhecido"
	}

	return statusStr, statusInt, titulo, nil
}

// mapaStatusGLPI mapeia os códigos numéricos do GLPI para descrições legíveis.
var mapaStatusGLPI = map[int]string{
	1: "⚪ Em Aberto",
	2: "🟡 Em Atendimento",
	3: "🟡 Em Atendimento (Planejado)",
	4: "🟠 Pendente / Aguardando Resposta",
	5: "🟢 Solucionado",
	6: "⚫ Fechado",
}

// extrairStatusInt lida com a diversidade de tipos que o campo "status" pode ter na API.

// Função extrairStatusInt executa a regra de negócio/rotina correspondente
func extrairStatusInt(statusVal interface{}) int {
	switch val := statusVal.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		n, _ := strconv.Atoi(val)
		return n
	case map[string]interface{}:
		return extrairStatusInt(val["id"])
	}
	return 0
}

// AdicionarMensagemChamado acrescenta um followup (acompanhamento) a um ticket existente.

// Função AdicionarMensagemChamado executa a regra de negócio/rotina correspondente
func AdicionarMensagemChamado(sessionToken string, ticketID int, mensagem string) error {
	cfg := config.GetConfig()
	urlStr := fmt.Sprintf("%s/ITILFollowup", getBaseURL())

	// Converte quebras de linha para HTML (padrão do GLPI)
	mensagemHTML := strings.ReplaceAll(mensagem, "\n", "<br>")

	payload := map[string]interface{}{
		"input": map[string]interface{}{
			"itemtype": "Ticket",
			"items_id": ticketID,
			"content":  mensagemHTML,
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	setCommonHeaders(req, cfg.GLPIAppToken, sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("erro ao adicionar mensagem (HTTP %d): %s", resp.StatusCode, body)
	}

	return nil
}

// ─── Documentos ───────────────────────────────────────────────────────────────

// AnexarDocumento faz o upload de um arquivo e o associa a um ticket no GLPI.

// Função AnexarDocumento executa a regra de negócio/rotina correspondente
func AnexarDocumento(sessionToken string, ticketID int, fileBytes []byte, filename string) error {
	cfg := config.GetConfig()
	urlStr := fmt.Sprintf("%s/Document", getBaseURL())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Manifesto obrigatório pela API do GLPI
	manifestPart, _ := writer.CreateFormField("uploadManifest")
	manifest := fmt.Sprintf(
		`{"input": {"name": "Anexo via WhatsApp", "_filename": ["%s"], "itemtype": "Ticket", "items_id": %d}}`,
		filename, ticketID,
	)
	manifestPart.Write([]byte(manifest))

	filePart, _ := writer.CreateFormFile(filename, filename)
	filePart.Write(fileBytes)
	writer.Close()

	req, err := http.NewRequest(http.MethodPost, urlStr, body)
	if err != nil {
		return err
	}
	req.Header.Set("App-Token", cfg.GLPIAppToken)
	req.Header.Set("Session-Token", sessionToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("erro ao anexar documento (HTTP %d)", resp.StatusCode)
	}

	return nil
}

// ─── Usuários ─────────────────────────────────────────────────────────────────

// BuscarUsuariosPorNome pesquisa usuários no GLPI por nome, sobrenome ou login.
// Retorna até 10 resultados, excluindo visitantes automáticos.

// Função BuscarUsuariosPorNome executa a regra de negócio/rotina correspondente
func BuscarUsuariosPorNome(sessionToken, nomeBusca string) ([]string, []int, error) {
	cfg := config.GetConfig()

	q := url.QueryEscape(nomeBusca)
	urlBusca := fmt.Sprintf(
		"%s/search/User?criteria[0][field]=9&criteria[0][searchtype]=contains&criteria[0][value]=%s"+
			"&criteria[1][link]=OR&criteria[1][field]=34&criteria[1][searchtype]=contains&criteria[1][value]=%s"+
			"&criteria[2][link]=OR&criteria[2][field]=1&criteria[2][searchtype]=contains&criteria[2][value]=%s"+
			"&forcedisplay[0]=1&forcedisplay[1]=9&forcedisplay[2]=2&forcedisplay[3]=34&range=0-30",
		getBaseURL(), q, q, q,
	)

	req, err := http.NewRequest(http.MethodGet, urlBusca, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("App-Token", cfg.GLPIAppToken)
	req.Header.Set("Session-Token", sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("erro na busca de usuários (HTTP %d)", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	rows, err := extrairLinhasUsuario(bodyBytes)
	if err != nil {
		return nil, nil, err
	}

	return processarLinhasUsuario(rows)
}

// extrairLinhasUsuario normaliza a resposta da API (array ou objeto com "data")
// em uma slice de mapas uniforme.

// Função extrairLinhasUsuario executa a regra de negócio/rotina correspondente
func extrairLinhasUsuario(bodyBytes []byte) ([]map[string]interface{}, error) {
	var raw interface{}
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		return nil, err
	}

	var rows []map[string]interface{}
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				rows = append(rows, m)
			}
		}
	case map[string]interface{}:
		if dataSlice, ok := v["data"].([]interface{}); ok {
			for _, item := range dataSlice {
				if m, ok := item.(map[string]interface{}); ok {
					rows = append(rows, m)
				}
			}
		}
	}

	return rows, nil
}

// processarLinhasUsuario extrai nomes e IDs da lista de linhas retornadas pela busca,
// eliminando duplicatas e usuários visitantes gerados automaticamente.

// Função processarLinhasUsuario executa a regra de negócio/rotina correspondente
func processarLinhasUsuario(rows []map[string]interface{}) ([]string, []int, error) {
	var nomes []string
	var ids []int
	seen := make(map[string]bool)

	for _, u := range rows {
		username, _ := u["1"].(string)
		if strings.HasPrefix(username, "visitante_") {
			continue
		}

		nomePrincipal, _ := u["9"].(string)
		sobrenome, _ := u["34"].(string)
		idStr := fmt.Sprintf("%v", u["2"])
		id, _ := strconv.Atoi(idStr)

		nomeFinal := montarNome(nomePrincipal, sobrenome, username)
		if nomeFinal == "" || seen[nomeFinal] {
			continue
		}

		seen[nomeFinal] = true
		nomes = append(nomes, nomeFinal)
		ids = append(ids, id)

		if len(nomes) >= 10 {
			break
		}
	}

	return nomes, ids, nil
}

// montarNome combina primeiro nome e sobrenome; usa o username como fallback.

// Função montarNome executa a regra de negócio/rotina correspondente
func montarNome(nomePrincipal, sobrenome, username string) string {
	if nomePrincipal != "" {
		if sobrenome != "" && sobrenome != nomePrincipal {
			return nomePrincipal + " " + sobrenome
		}
		return nomePrincipal
	}
	return username
}

// CriarUsuarioVisitante cria um usuário temporário no GLPI para solicitantes não cadastrados.

// Função CriarUsuarioVisitante executa a regra de negócio/rotina correspondente
func CriarUsuarioVisitante(sessionToken, nomeCompleto string) (int, error) {
	cfg := config.GetConfig()
	urlStr := fmt.Sprintf("%s/User", getBaseURL())

	partes := strings.SplitN(nomeCompleto, " ", 2)
	primeiroNome := partes[0]
	sobrenome := ""
	if len(partes) > 1 {
		sobrenome = partes[1]
	}

	payload := map[string]interface{}{
		"input": map[string]interface{}{
			"name":      fmt.Sprintf("visitante_%d", time.Now().Unix()),
			"firstname": primeiroNome,
			"realname":  sobrenome,
			"is_active": 1,
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}
	setCommonHeaders(req, cfg.GLPIAppToken, sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("erro ao criar usuário visitante: %s", body)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if idFloat, ok := result["id"].(float64); ok {
		return int(idFloat), nil
	}

	return 0, fmt.Errorf("id do usuário não retornado")
}

// ─── Integração e Autenticação de Usuários GLPI ───────────────────────────────

// Struct GLPIUserDTO representa os dados básicos de um usuário cadastrado no GLPI
type GLPIUserDTO struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	RealName  string `json:"realname"`
	FirstName string `json:"firstname"`
}

// BuscarUsuariosGLPI consulta a API REST do GLPI e retorna a lista de usuários cadastrados
func BuscarUsuariosGLPI(sessionToken string) ([]GLPIUserDTO, error) {
	cfg := config.GetConfig()
	apiURL := getBaseURL()

	// 1. Tenta GET /User?range=0-500
	urlStr := fmt.Sprintf("%s/User?range=0-500", apiURL)
	fmt.Printf("🔍 [GLPI API] Buscando usuários em: %s ...\n", urlStr)

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err == nil {
		setCommonHeaders(req, cfg.GLPIAppToken, sessionToken)
		resp, errDo := httpClient.Do(req)
		if errDo == nil {
			defer resp.Body.Close()
			bodyBytes, _ := io.ReadAll(resp.Body)
			fmt.Printf("📊 [GLPI API /User] Status HTTP %d (Tamanho: %d bytes)\n", resp.StatusCode, len(bodyBytes))

			if resp.StatusCode == http.StatusOK || resp.StatusCode == 206 {
				rows, errExt := extrairLinhasUsuario(bodyBytes)
				if errExt == nil && len(rows) > 0 {
					var result []GLPIUserDTO
					for _, item := range rows {
						var u GLPIUserDTO
						if idVal, ok := item["id"].(float64); ok {
							u.ID = int(idVal)
						} else if idStr := fmt.Sprintf("%v", item["2"]); idStr != "" && idStr != "<nil>" {
							u.ID, _ = strconv.Atoi(idStr)
						} else if idStr := fmt.Sprintf("%v", item["id"]); idStr != "" && idStr != "<nil>" {
							u.ID, _ = strconv.Atoi(idStr)
						}

						if nameVal, ok := item["name"].(string); ok {
							u.Username = nameVal
						} else if userVal, ok := item["1"].(string); ok {
							u.Username = userVal
						}

						if realVal, ok := item["realname"].(string); ok {
							u.RealName = realVal
						} else if realVal, ok := item["34"].(string); ok {
							u.RealName = realVal
						}

						if firstVal, ok := item["firstname"].(string); ok {
							u.FirstName = firstVal
						} else if firstVal, ok := item["9"].(string); ok {
							u.FirstName = firstVal
						}

						u.Name = montarNome(u.FirstName, u.RealName, u.Username)
						if u.Username != "" && u.ID > 0 && !strings.HasPrefix(u.Username, "visitante_") {
							result = append(result, u)
						}
					}
					if len(result) > 0 {
						fmt.Printf("✅ [GLPI API] %d usuários carregados via /User!\n", len(result))
						return result, nil
					}
				}
			} else {
				fmt.Printf("⚠️ [GLPI API /User] Rejeitado pelo GLPI: %s\n", string(bodyBytes))
			}
		} else {
			fmt.Printf("⚠️ [GLPI API /User] Erro de rede: %v\n", errDo)
		}
	}

	// 2. Fallback para GET /search/User
	searchURL := fmt.Sprintf("%s/search/User?forcedisplay[0]=1&forcedisplay[1]=2&forcedisplay[2]=9&forcedisplay[3]=34&range=0-500", apiURL)
	fmt.Printf("🔍 [GLPI API] Tentando fallback via Search API em: %s ...\n", searchURL)
	reqSearch, errS := http.NewRequest(http.MethodGet, searchURL, nil)
	if errS != nil {
		return nil, errS
	}
	setCommonHeaders(reqSearch, cfg.GLPIAppToken, sessionToken)

	respSearch, errDS := httpClient.Do(reqSearch)
	if errDS != nil {
		return nil, fmt.Errorf("erro de conexão no fallback /search/User: %v", errDS)
	}
	defer respSearch.Body.Close()

	bodyBytes, _ := io.ReadAll(respSearch.Body)
	fmt.Printf("📊 [GLPI API /search/User] Status HTTP %d (Tamanho: %d bytes)\n", respSearch.StatusCode, len(bodyBytes))

	if respSearch.StatusCode != http.StatusOK && respSearch.StatusCode != 206 {
		return nil, fmt.Errorf("GLPI recusou busca /search/User (HTTP %d): %s", respSearch.StatusCode, string(bodyBytes))
	}

	rows, errExt := extrairLinhasUsuario(bodyBytes)
	if errExt != nil {
		return nil, fmt.Errorf("erro ao estruturar usuários: %v", errExt)
	}

	var result []GLPIUserDTO
	for _, item := range rows {
		var u GLPIUserDTO
		if userVal, ok := item["1"].(string); ok {
			u.Username = userVal
		}
		if idStr := fmt.Sprintf("%v", item["2"]); idStr != "" && idStr != "<nil>" {
			u.ID, _ = strconv.Atoi(idStr)
		}
		if firstVal, ok := item["9"].(string); ok {
			u.FirstName = firstVal
		}
		if realVal, ok := item["34"].(string); ok {
			u.RealName = realVal
		}

		if u.ID == 0 {
			if idVal, ok := item["id"].(float64); ok {
				u.ID = int(idVal)
			}
		}
		if u.Username == "" {
			if nameVal, ok := item["name"].(string); ok {
				u.Username = nameVal
			}
		}

		u.Name = montarNome(u.FirstName, u.RealName, u.Username)
		if u.Username != "" && u.ID > 0 && !strings.HasPrefix(u.Username, "visitante_") {
			result = append(result, u)
		}
	}

	fmt.Printf("✅ [GLPI API] %d usuários carregados via /search/User!\n", len(result))
	return result, nil
}

// AutenticarUsuarioGLPI tenta autenticar as credenciais informadas diretamente na API do GLPI
func AutenticarUsuarioGLPI(username, password string) (bool, error) {
	cfg := config.GetConfig()
	rawURL := strings.TrimSpace(cfg.GLPIApiURL)
	appToken := strings.TrimSpace(cfg.GLPIAppToken)

	if rawURL == "" || appToken == "" {
		return false, fmt.Errorf("URL ou App-Token da API do GLPI não configurados")
	}

	authStr := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	userAuthHeader := "Basic " + authStr

	candidates := []string{strings.TrimSuffix(rawURL, "/")}
	if idx := strings.Index(rawURL, "/api.php"); idx != -1 {
		baseDomain := rawURL[:idx]
		candidates = append(candidates, baseDomain+"/apirest.php")
	} else if !strings.Contains(rawURL, "apirest.php") {
		candidates = append(candidates, strings.TrimSuffix(rawURL, "/")+"/apirest.php")
	}

	var lastErr error
	for _, cand := range candidates {
		// Modo A: Via Header Basic Authorization
		urlStr := fmt.Sprintf("%s/initSession", cand)
		fmt.Printf("🔐 [GLPI AUTH A] Autenticando '@%s' via Basic Header em %s ...\n", username, urlStr)

		req, err := http.NewRequest(http.MethodGet, urlStr, nil)
		if err == nil {
			req.Header.Set("App-Token", appToken)
			req.Header.Set("Authorization", userAuthHeader)
			req.Header.Set("Content-Type", "application/json")

			resp, errDo := httpClient.Do(req)
			if errDo == nil {
				bodyBytes, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				fmt.Printf("📊 [GLPI AUTH A] Status HTTP %d (Tamanho: %d bytes)\n", resp.StatusCode, len(bodyBytes))

				if resp.StatusCode == http.StatusOK {
					var resMap map[string]interface{}
					if err := json.Unmarshal(bodyBytes, &resMap); err == nil {
						if sessToken, ok := resMap["session_token"].(string); ok && sessToken != "" {
							closeURL := fmt.Sprintf("%s/killSession", cand)
							cReq, _ := http.NewRequest(http.MethodGet, closeURL, nil)
							setCommonHeaders(cReq, appToken, sessToken)
							_, _ = httpClient.Do(cReq)
						}
					}
					fmt.Printf("✅ [GLPI AUTH] Usuário '@%s' autenticado com sucesso no GLPI!\n", username)
					return true, nil
				}
				fmt.Printf("⚠️ [GLPI AUTH A] GLPI retornou HTTP %d: %s\n", resp.StatusCode, string(bodyBytes))
				lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
			} else {
				lastErr = errDo
			}
		}

		// Modo B: Via Query Params (?login=...&password=...) caso o Nginx/Apache remova o header Authorization
		urlQueryStr := fmt.Sprintf("%s/initSession?login=%s&password=%s", cand, url.QueryEscape(username), url.QueryEscape(password))
		fmt.Printf("🔐 [GLPI AUTH B] Autenticando '@%s' via Query Params em %s ...\n", username, cand)

		reqB, errB := http.NewRequest(http.MethodGet, urlQueryStr, nil)
		if errB == nil {
			reqB.Header.Set("App-Token", appToken)
			reqB.Header.Set("Content-Type", "application/json")

			respB, errDoB := httpClient.Do(reqB)
			if errDoB == nil {
				bodyBytesB, _ := io.ReadAll(respB.Body)
				respB.Body.Close()
				fmt.Printf("📊 [GLPI AUTH B] Status HTTP %d (Tamanho: %d bytes)\n", respB.StatusCode, len(bodyBytesB))

				if respB.StatusCode == http.StatusOK {
					var resMap map[string]interface{}
					if err := json.Unmarshal(bodyBytesB, &resMap); err == nil {
						if sessToken, ok := resMap["session_token"].(string); ok && sessToken != "" {
							closeURL := fmt.Sprintf("%s/killSession", cand)
							cReq, _ := http.NewRequest(http.MethodGet, closeURL, nil)
							setCommonHeaders(cReq, appToken, sessToken)
							_, _ = httpClient.Do(cReq)
						}
					}
					fmt.Printf("✅ [GLPI AUTH] Usuário '@%s' autenticado com sucesso via Query Params!\n", username)
					return true, nil
				}
				fmt.Printf("⚠️ [GLPI AUTH B] GLPI retornou HTTP %d: %s\n", respB.StatusCode, string(bodyBytesB))
				lastErr = fmt.Errorf("HTTP %d: %s", respB.StatusCode, string(bodyBytesB))
			}
		}
	}

	return false, lastErr
}

