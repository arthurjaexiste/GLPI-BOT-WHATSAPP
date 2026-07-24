// ============================================================================
// ARQUIVO: glpi.go
// Descrição: Implementação Go (backend) para o ecossistema GLPI-BOT.
// ============================================================================

package glpi

import (
	"bytes"
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

// httpClient é o cliente HTTP compartilhado (sem tempo limite configurado aqui,
// mas reutilizado para melhor desempenho de conexões keep-alive).
var httpClient = &http.Client{}

// getBaseURL retorna a URL base da API do GLPI sem barra final.

// Função getBaseURL executa a regra de negócio/rotina correspondente
func getBaseURL() string {
	return strings.TrimSuffix(config.GetConfig().GLPIApiURL, "/")
}

// setCommonHeaders adiciona os cabeçalhos obrigatórios para todas as requisições GLPI.

// Função setCommonHeaders executa a regra de negócio/rotina correspondente
func setCommonHeaders(req *http.Request, appToken, sessionToken string) {
	req.Header.Set("App-Token", appToken)
	req.Header.Set("Content-Type", "application/json")
	if sessionToken != "" {
		req.Header.Set("Session-Token", sessionToken)
	}
}

// ─── Sessão ───────────────────────────────────────────────────────────────────

// GetGLPISession inicia uma sessão na API do GLPI e retorna o session_token.

// Função GetGLPISession executa a regra de negócio/rotina correspondente
func GetGLPISession() (string, error) {
	cfg := config.GetConfig()
	urlStr := fmt.Sprintf("%s/initSession", getBaseURL())

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		return "", err
	}

	userToken := cfg.GLPIUserToken
	if !strings.HasPrefix(strings.ToLower(userToken), "user_token ") {
		userToken = "user_token " + userToken
	}

	req.Header.Set("App-Token", cfg.GLPIAppToken)
	req.Header.Set("Authorization", userToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("falha na autenticação (HTTP %d): %s", resp.StatusCode, body)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	token, ok := result["session_token"].(string)
	if !ok {
		return "", fmt.Errorf("token de sessão não encontrado na resposta")
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
