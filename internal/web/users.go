package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bot-glpi/internal/database"
	"bot-glpi/internal/glpi"
)

// UserDTO estrutura os dados de resposta para usuários do sistema
type UserDTO struct {
	ID        int    `json:"id"`
	GLPIID    int    `json:"glpi_id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

// HandleUsersAPI gerencia CRUD completo de usuários
func HandleUsersAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetUserSession(r)
		if !ok {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}
		if session.Role != "admin" {
			http.Error(w, "Acesso negado: Requer perfil de Administrador", http.StatusForbidden)
			return
		}

		if database.DB == nil {
			http.Error(w, "Banco de dados indisponível", http.StatusInternalServerError)
			return
		}

		switch r.Method {
		case "GET":
			rows, err := database.DB.Query("SELECT id, COALESCE(glpi_id, 0), username, COALESCE(name, 'Usuário'), COALESCE(email, ''), role, enabled, created_at FROM users ORDER BY id ASC")
			if err != nil {
				http.Error(w, fmt.Sprintf("Erro ao buscar usuários: %v", err), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			usersList := []UserDTO{}
			for rows.Next() {
				var u UserDTO
				var enabledInt int
				var rawTime string
				if err := rows.Scan(&u.ID, &u.GLPIID, &u.Username, &u.Name, &u.Email, &u.Role, &enabledInt, &rawTime); err == nil {
					u.Enabled = (enabledInt == 1)
					if parsed, errTime := time.Parse("2006-01-02 15:04:05", rawTime); errTime == nil {
						loc, _ := time.LoadLocation("America/Sao_Paulo")
						if loc != nil {
							parsed = parsed.In(loc)
						}
						u.CreatedAt = parsed.Format("02/01/2006 15:04:05")
					} else {
						u.CreatedAt = rawTime
					}
					usersList = append(usersList, u)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(usersList)

		case "POST":
			var req struct {
				Username string `json:"username"`
				Name     string `json:"name"`
				Email    string `json:"email"`
				Password string `json:"password"`
				Role     string `json:"role"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Requisição inválida", http.StatusBadRequest)
				return
			}

			req.Username = strings.TrimSpace(req.Username)
			req.Name = strings.TrimSpace(req.Name)
			req.Email = strings.TrimSpace(req.Email)
			req.Password = strings.TrimSpace(req.Password)
			if req.Role != "admin" && req.Role != "operator" {
				req.Role = "operator"
			}

			if req.Username == "" || req.Password == "" || req.Name == "" {
				http.Error(w, "Nome, usuário e senha são obrigatórios.", http.StatusBadRequest)
				return
			}

			hash, err := database.HashPassword(req.Password)
			if err != nil {
				http.Error(w, "Erro ao criptografar senha", http.StatusInternalServerError)
				return
			}

			_, err = database.DB.Exec("INSERT INTO users (username, password, name, email, role) VALUES (?, ?, ?, ?, ?)", req.Username, hash, req.Name, req.Email, req.Role)
			if err != nil {
				if strings.Contains(err.Error(), "UNIQUE") {
					http.Error(w, "Nome de usuário já existe.", http.StatusConflict)
					return
				}
				http.Error(w, fmt.Sprintf("Erro ao cadastrar usuário: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Usuário criado com sucesso"})

		case "PUT":
			var req struct {
				ID       int    `json:"id"`
				Username string `json:"username"`
				Name     string `json:"name"`
				Email    string `json:"email"`
				Password string `json:"password"`
				Role     string `json:"role"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "Requisição inválida", http.StatusBadRequest)
				return
			}

			req.Username = strings.TrimSpace(req.Username)
			req.Name = strings.TrimSpace(req.Name)
			req.Email = strings.TrimSpace(req.Email)
			if req.Role != "admin" && req.Role != "operator" {
				req.Role = "operator"
			}

			if req.ID <= 0 || req.Username == "" || req.Name == "" {
				http.Error(w, "ID, nome e usuário são obrigatórios.", http.StatusBadRequest)
				return
			}

			if req.Password != "" {
				hash, err := database.HashPassword(req.Password)
				if err != nil {
					http.Error(w, "Erro ao criptografar senha", http.StatusInternalServerError)
					return
				}
				_, err = database.DB.Exec("UPDATE users SET username = ?, name = ?, email = ?, role = ?, password = ? WHERE id = ?", req.Username, req.Name, req.Email, req.Role, hash, req.ID)
				if err != nil {
					http.Error(w, fmt.Sprintf("Erro ao atualizar usuário: %v", err), http.StatusInternalServerError)
					return
				}
			} else {
				_, err := database.DB.Exec("UPDATE users SET username = ?, name = ?, email = ?, role = ? WHERE id = ?", req.Username, req.Name, req.Email, req.Role, req.ID)
				if err != nil {
					http.Error(w, fmt.Sprintf("Erro ao atualizar usuário: %v", err), http.StatusInternalServerError)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Usuário atualizado com sucesso"})

		case "DELETE":
			idStr := r.URL.Query().Get("id")
			id, err := strconv.Atoi(idStr)
			if err != nil || id <= 0 {
				http.Error(w, "ID inválido", http.StatusBadRequest)
				return
			}

			if id == session.UserID {
				http.Error(w, "Você não pode excluir o seu próprio usuário logado.", http.StatusBadRequest)
				return
			}

			var adminCount int
			_ = database.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&adminCount)

			var targetRole string
			_ = database.DB.QueryRow("SELECT role FROM users WHERE id = ?", id).Scan(&targetRole)

			if targetRole == "admin" && adminCount <= 1 {
				http.Error(w, "Não é possível excluir o único Administrador do sistema.", http.StatusBadRequest)
				return
			}

			_, err = database.DB.Exec("DELETE FROM users WHERE id = ?", id)
			if err != nil {
				http.Error(w, fmt.Sprintf("Erro ao excluir usuário: %v", err), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Usuário removido com sucesso"})

		default:
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
		}
	}
}

// HandleGLPIUsersAPI lista usuários integrados do GLPI e mescla com acessos no banco local
func HandleGLPIUsersAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetUserSession(r)
		if !ok {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}
		if session.Role != "admin" {
			http.Error(w, "Acesso negado: Requer perfil de Administrador", http.StatusForbidden)
			return
		}

		if database.DB == nil {
			http.Error(w, "Banco de dados indisponível", http.StatusInternalServerError)
			return
		}

		sessToken, errSess := glpi.GetGLPISession()
		if errSess != nil {
			http.Error(w, fmt.Sprintf("Erro ao conectar no GLPI: %v. Verifique a URL e os Tokens da API nas Configurações.", errSess), http.StatusBadRequest)
			return
		}

		glpiUsers, errGLPI := glpi.BuscarUsuariosGLPI(sessToken)
		if errGLPI != nil {
			http.Error(w, fmt.Sprintf("Erro ao listar usuários do GLPI: %v", errGLPI), http.StatusBadRequest)
			return
		}

		rows, err := database.DB.Query("SELECT id, COALESCE(glpi_id, 0), username, name, role, enabled FROM users")
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao buscar usuários locais: %v", err), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type LocalUser struct {
			ID       int
			GLPIID   int
			Username string
			Name     string
			Role     string
			Enabled  int
		}

		localMap := make(map[string]LocalUser)
		for rows.Next() {
			var lu LocalUser
			if err := rows.Scan(&lu.ID, &lu.GLPIID, &lu.Username, &lu.Name, &lu.Role, &lu.Enabled); err == nil {
				localMap[strings.ToLower(lu.Username)] = lu
			}
		}

		type CombinedUserDTO struct {
			ID       int    `json:"id"`
			GLPIID   int    `json:"glpi_id"`
			Username string `json:"username"`
			Name     string `json:"name"`
			Role     string `json:"role"`
			Enabled  bool   `json:"enabled"`
			InDB     bool   `json:"in_db"`
		}

		combinedList := []CombinedUserDTO{}
		seenUsernames := make(map[string]bool)

		for _, gu := range glpiUsers {
			lowerUser := strings.ToLower(gu.Username)
			seenUsernames[lowerUser] = true

			dto := CombinedUserDTO{
				GLPIID:   gu.ID,
				Username: gu.Username,
				Name:     gu.Name,
				Role:     "operator",
				Enabled:  false,
				InDB:     false,
			}

			if lu, exists := localMap[lowerUser]; exists {
				dto.ID = lu.ID
				dto.Role = lu.Role
				dto.Enabled = (lu.Enabled == 1)
				dto.InDB = true
				if gu.Name != "" {
					dto.Name = gu.Name
				}
			}

			combinedList = append(combinedList, dto)
		}

		for lowerUser, lu := range localMap {
			if !seenUsernames[lowerUser] {
				combinedList = append(combinedList, CombinedUserDTO{
					ID:       lu.ID,
					GLPIID:   lu.GLPIID,
					Username: lu.Username,
					Name:     lu.Name,
					Role:     lu.Role,
					Enabled:  (lu.Enabled == 1),
					InDB:     true,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(combinedList)
	}
}

// HandleToggleAccessAPI habilita/desabilita o acesso de um usuário do GLPI
func HandleToggleAccessAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetUserSession(r)
		if !ok {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}
		if session.Role != "admin" {
			http.Error(w, "Acesso negado: Requer perfil de Administrador", http.StatusForbidden)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			GLPIID   int    `json:"glpi_id"`
			Username string `json:"username"`
			Name     string `json:"name"`
			Password string `json:"password"`
			Role     string `json:"role"`
			Enabled  bool   `json:"enabled"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Requisição inválida", http.StatusBadRequest)
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		req.Name = strings.TrimSpace(req.Name)
		req.Password = strings.TrimSpace(req.Password)
		if req.Role != "admin" && req.Role != "operator" {
			req.Role = "operator"
		}

		if req.Username == "" {
			http.Error(w, "Nome de usuário é obrigatório.", http.StatusBadRequest)
			return
		}

		enabledInt := 0
		if req.Enabled {
			enabledInt = 1
		}

		if strings.EqualFold(req.Username, session.Username) && !req.Enabled {
			http.Error(w, "Você não pode desativar o seu próprio acesso logado.", http.StatusBadRequest)
			return
		}

		passToSave := "glpi_user"
		if req.Password != "" {
			if hash, errH := database.HashPassword(req.Password); errH == nil {
				passToSave = hash
			}
		}

		var existingID int
		err := database.DB.QueryRow("SELECT id FROM users WHERE LOWER(username) = LOWER(?)", req.Username).Scan(&existingID)
		if err == nil {
			if req.Password != "" {
				_, err = database.DB.Exec("UPDATE users SET glpi_id = ?, name = ?, role = ?, enabled = ?, password = ? WHERE id = ?", req.GLPIID, req.Name, req.Role, enabledInt, passToSave, existingID)
			} else {
				_, err = database.DB.Exec("UPDATE users SET glpi_id = ?, name = ?, role = ?, enabled = ? WHERE id = ?", req.GLPIID, req.Name, req.Role, enabledInt, existingID)
			}
		} else {
			_, err = database.DB.Exec("INSERT INTO users (glpi_id, username, password, name, role, enabled) VALUES (?, ?, ?, ?, ?, ?)", req.GLPIID, req.Username, passToSave, req.Name, req.Role, enabledInt)
		}

		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao salvar permissão: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Permissão atualizada com sucesso"})
	}
}

// HandleDeleteUserAPI remove o registro de um usuário no banco do bot
func HandleDeleteUserAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetUserSession(r)
		if !ok {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}
		if session.Role != "admin" {
			http.Error(w, "Acesso negado", http.StatusForbidden)
			return
		}
		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Requisição inválida", http.StatusBadRequest)
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		if strings.EqualFold(req.Username, session.Username) {
			http.Error(w, "Você não pode remover a si mesmo do sistema.", http.StatusBadRequest)
			return
		}

		var adminCount int
		_ = database.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&adminCount)

		var targetRole string
		_ = database.DB.QueryRow("SELECT role FROM users WHERE LOWER(username) = LOWER(?)", req.Username).Scan(&targetRole)

		if targetRole == "admin" && adminCount <= 1 {
			http.Error(w, "Não é possível excluir o único Administrador do sistema.", http.StatusBadRequest)
			return
		}

		_, err := database.DB.Exec("DELETE FROM users WHERE LOWER(username) = LOWER(?)", req.Username)
		if err != nil {
			http.Error(w, fmt.Sprintf("Erro ao remover usuário: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Usuário removido com sucesso"})
	}
}

// HandleChangePasswordAPI altera a senha do usuário autenticado atualmente na sessão
func HandleChangePasswordAPI() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := GetUserSession(r)
		if !ok {
			http.Error(w, "Não autorizado", http.StatusUnauthorized)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Método não permitido", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "JSON inválido", http.StatusBadRequest)
			return
		}

		if req.NewPassword == "" {
			http.Error(w, "A nova senha não pode ser vazia", http.StatusBadRequest)
			return
		}

		if len(req.NewPassword) < 6 {
			http.Error(w, "A nova senha deve ter no mínimo 6 caracteres", http.StatusBadRequest)
			return
		}

		var dbPass string
		err := database.DB.QueryRow("SELECT password FROM users WHERE id = ?", session.UserID).Scan(&dbPass)
		if err != nil {
			http.Error(w, "Erro ao buscar senha atual", http.StatusInternalServerError)
			return
		}

		if !database.CheckPasswordHash(req.CurrentPassword, dbPass) && dbPass != req.CurrentPassword {
			http.Error(w, "Senha atual incorreta", http.StatusBadRequest)
			return
		}

		newHash, err := database.HashPassword(req.NewPassword)
		if err != nil {
			http.Error(w, "Erro ao criptografar nova senha", http.StatusInternalServerError)
			return
		}

		_, err = database.DB.Exec("UPDATE users SET password = ? WHERE id = ?", newHash, session.UserID)
		if err != nil {
			http.Error(w, "Erro ao atualizar senha no banco de dados", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
