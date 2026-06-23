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

func getBaseURL() string {
	urlStr := config.GetConfig().GLPIApiURL
	return strings.TrimSuffix(urlStr, "/")
}

func GetGLPISession() (string, error) {
	cfg := config.GetConfig()
	urlStr := fmt.Sprintf("%s/initSession", getBaseURL())
	
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil { return "", err }

	req.Header.Set("App-Token", cfg.GLPIAppToken)
	
	userToken := cfg.GLPIUserToken
	if !strings.HasPrefix(strings.ToLower(userToken), "user_token ") {
		userToken = "user_token " + userToken
	}
	
	req.Header.Set("Authorization", userToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("falha na autenticação (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	token, ok := result["session_token"].(string)
	if !ok { return "", fmt.Errorf("token de sessao nao encontrado na resposta") }

	return token, nil
}

func CriarChamado(sessionToken string, titulo string, descricao string, urgencia int, requesterID int, categoryID int) (int, error) {
	urlStr := fmt.Sprintf("%s/Ticket", getBaseURL())

	inputData := map[string]interface{}{
		"name":     titulo,
		"content":  descricao,
		"priority": urgencia, 
	}

	if requesterID > 0 {
		inputData["_users_id_requester"] = requesterID
	}

	if categoryID > 0 {
		inputData["itilcategories_id"] = categoryID
	}

	payload := map[string]interface{}{
		"input": inputData,
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(jsonData))
	if err != nil { return 0, err }
	
	req.Header.Set("App-Token", config.GetConfig().GLPIAppToken)
	req.Header.Set("Session-Token", sessionToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return 0, err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("erro ao criar chamado (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	if idFloat, ok := result["id"].(float64); ok {
		return int(idFloat), nil
	}

	return 0, fmt.Errorf("id do chamado não retornado")
}

func AnexarDocumento(sessionToken string, ticketID int, fileBytes []byte, filename string) error {
	urlStr := fmt.Sprintf("%s/Document", getBaseURL())

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	manifestPart, _ := writer.CreateFormField("uploadManifest")
	manifest := fmt.Sprintf(`{"input": {"name": "Anexo via WhatsApp", "_filename": ["%s"], "itemtype": "Ticket", "items_id": %d}}`, filename, ticketID)
	manifestPart.Write([]byte(manifest))

	filePart, _ := writer.CreateFormFile(filename, filename)
	filePart.Write(fileBytes)
	writer.Close()

	req, err := http.NewRequest("POST", urlStr, body)
	if err != nil { return err }

	req.Header.Set("App-Token", config.GetConfig().GLPIAppToken)
	req.Header.Set("Session-Token", sessionToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("erro ao anexar documento: %d", resp.StatusCode)
	}

	return nil
}

func BuscarUsuariosPorNome(sessionToken string, nomeBusca string) ([]string, []int, error) {
	query := url.QueryEscape(nomeBusca)
	urlBusca := fmt.Sprintf("%s/search/User?criteria[0][field]=9&criteria[0][searchtype]=contains&criteria[0][value]=%s&criteria[1][link]=OR&criteria[1][field]=34&criteria[1][searchtype]=contains&criteria[1][value]=%s&criteria[2][link]=OR&criteria[2][field]=1&criteria[2][searchtype]=contains&criteria[2][value]=%s&forcedisplay[0]=1&forcedisplay[1]=9&forcedisplay[2]=2&forcedisplay[3]=34&range=0-30", getBaseURL(), query, query, query)
	
	req, _ := http.NewRequest("GET", urlBusca, nil)
	req.Header.Set("App-Token", config.GetConfig().GLPIAppToken)
	req.Header.Set("Session-Token", sessionToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return nil, nil, err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("erro na busca: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil { return nil, nil, err }

	var rawResponse interface{}
	if err := json.Unmarshal(bodyBytes, &rawResponse); err != nil {
		return nil, nil, err
	}

	var rows []map[string]interface{}
	switch v := rawResponse.(type) {
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

	var nomesEncontrados []string
	var idsEncontrados []int
	mapaNomes := make(map[string]bool)

	for _, u := range rows {
		username, _ := u["1"].(string)       
		if strings.HasPrefix(username, "visitante_") {
			continue
		}

		nomeFinal := ""
		nomePrincipal, _ := u["9"].(string)  
		sobrenome, _ := u["34"].(string)     
		
		idStr := fmt.Sprintf("%v", u["2"])
		id, _ := strconv.Atoi(idStr)
		
		if nomePrincipal != "" {
			nomeFinal = nomePrincipal
			if sobrenome != "" && sobrenome != nomePrincipal {
				nomeFinal += " " + sobrenome
			}
		} else if username != "" {
			nomeFinal = username
		}
		
		if nomeFinal != "" && !mapaNomes[nomeFinal] {
			mapaNomes[nomeFinal] = true
			nomesEncontrados = append(nomesEncontrados, nomeFinal)
			idsEncontrados = append(idsEncontrados, id) 
		}
		
		if len(nomesEncontrados) >= 10 {
			break
		}
	}
	
	return nomesEncontrados, idsEncontrados, nil
}

func CriarUsuarioVisitante(sessionToken string, nomeCompleto string) (int, error) {
	urlStr := fmt.Sprintf("%s/User", getBaseURL())
	
	partes := strings.SplitN(nomeCompleto, " ", 2)
	primeiroNome := partes[0]
	sobrenome := ""
	if len(partes) > 1 {
		sobrenome = partes[1]
	}

	username := fmt.Sprintf("visitante_%d", time.Now().Unix())

	payload := map[string]interface{}{
		"input": map[string]interface{}{
			"name":      username,
			"firstname": primeiroNome,
			"realname":  sobrenome,
			"is_active": 1,
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(jsonData))
	if err != nil { return 0, err }
	
	req.Header.Set("App-Token", config.GetConfig().GLPIAppToken)
	req.Header.Set("Session-Token", sessionToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return 0, err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("erro ao criar usuario visitante: %s", string(bodyBytes))
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	
	if idFloat, ok := result["id"].(float64); ok {
		return int(idFloat), nil
	}

	return 0, fmt.Errorf("id do usuario nao retornado")
}

func BuscarChamado(sessionToken string, ticketID int) (string, int, string, error) {
	urlStr := fmt.Sprintf("%s/Ticket/%d", getBaseURL(), ticketID)
	
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("App-Token", config.GetConfig().GLPIAppToken)
	req.Header.Set("Session-Token", sessionToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return "", 0, "", err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", 0, "", fmt.Errorf("chamado não encontrado ou acesso negado")
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	var statusInt int
	if statusVal, ok := result["status"]; ok {
		switch val := statusVal.(type) {
		case float64:
			statusInt = int(val)
		case string:
			statusInt, _ = strconv.Atoi(val)
		case int:
			statusInt = val
		case map[string]interface{}:
			if idVal, ok := val["id"]; ok {
				switch id := idVal.(type) {
				case float64:
					statusInt = int(id)
				case string:
					statusInt, _ = strconv.Atoi(id)
				case int:
					statusInt = id
				}
			}
		}
	}
	titulo, _ := result["name"].(string)

	mapaStatus := map[int]string{
		1: "⚪ Em Aberto", 
		2: "🟡 Em Atendimento", 
		3: "🟡 Em Atendimento (Planejado)",
		4: "🟠 Pendente / Aguardando Resposta",
		5: "🟢 Solucionado",
		6: "⚫ Fechado",
	}

	statusStr := mapaStatus[statusInt]
	if statusStr == "" {
		statusStr = "Desconhecido"
	}

	return statusStr, statusInt, titulo, nil
}

func AdicionarMensagemChamado(sessionToken string, ticketID int, mensagem string) error {
	urlStr := fmt.Sprintf("%s/ITILFollowup", getBaseURL())

	mensagemFormatada := strings.ReplaceAll(mensagem, "\n", "<br>")

	payload := map[string]interface{}{
		"input": map[string]interface{}{
			"itemtype": "Ticket",
			"items_id": ticketID,
			"content":  mensagemFormatada,
		},
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", urlStr, bytes.NewBuffer(jsonData))
	if err != nil { return err }

	req.Header.Set("App-Token", config.GetConfig().GLPIAppToken)
	req.Header.Set("Session-Token", sessionToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("erro ao adicionar mensagem (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}