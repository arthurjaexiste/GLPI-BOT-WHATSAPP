package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// DB é a instância global de conexão com o banco de dados SQLite
var DB *sql.DB

// HashPassword gera o hash bcrypt para armazenamento seguro de senhas
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash compara a senha em texto com o hash bcrypt
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// InitDB inicializa a conexão SQLite e garante a criação de tabelas e migrações
func InitDB() error {
	_ = os.MkdirAll("db", 0777)
	var err error
	DB, err = sql.Open("sqlite", "db/web.db")
	if err != nil {
		fmt.Println("🚨 Erro ao iniciar DB:", err)
		return err
	}

	if err = DB.Ping(); err != nil {
		fmt.Println("🚨 Erro ao conectar no DB:", err)
		DB = nil
		return err
	}

	_, err = DB.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		glpi_id INTEGER UNIQUE,
		username TEXT UNIQUE NOT NULL,
		password TEXT,
		name TEXT NOT NULL DEFAULT 'Usuário',
		email TEXT,
		role TEXT NOT NULL DEFAULT 'operator',
		enabled INTEGER NOT NULL DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		fmt.Println("🚨 Erro ao criar tabela users:", err)
	}

	// Migrações seguras de colunas caso ainda não existam
	_, _ = DB.Exec(`ALTER TABLE users ADD COLUMN glpi_id INTEGER`)
	_, _ = DB.Exec(`ALTER TABLE users ADD COLUMN name TEXT NOT NULL DEFAULT 'Usuário'`)
	_, _ = DB.Exec(`ALTER TABLE users ADD COLUMN email TEXT`)
	_, _ = DB.Exec(`ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'operator'`)
	_, _ = DB.Exec(`ALTER TABLE users ADD COLUMN enabled INTEGER NOT NULL DEFAULT 1`)
	_, _ = DB.Exec(`ALTER TABLE users ADD COLUMN created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`)
	_, _ = DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_glpi_id ON users(glpi_id) WHERE glpi_id IS NOT NULL AND glpi_id > 0`)

	_, err = DB.Exec(`CREATE TABLE IF NOT EXISTS tickets_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		ticket_id TEXT, 
		title TEXT, 
		requester TEXT, 
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		fmt.Println("🚨 Erro ao criar tabela tickets_history:", err)
	}

	_, err = DB.Exec(`CREATE TABLE IF NOT EXISTS chat_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		chat_jid TEXT, 
		sender_name TEXT, 
		sender_jid TEXT, 
		message_text TEXT, 
		message_type TEXT, 
		is_from_me INTEGER, 
		media_url TEXT,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		fmt.Println("🚨 Erro ao criar tabela chat_messages:", err)
	}

	_, _ = DB.Exec(`ALTER TABLE chat_messages ADD COLUMN media_url TEXT`)
	_, _ = DB.Exec(`ALTER TABLE chat_messages ADD COLUMN reply_to_name TEXT`)
	_, _ = DB.Exec(`ALTER TABLE chat_messages ADD COLUMN reply_to_text TEXT`)
	_, _ = DB.Exec(`ALTER TABLE chat_messages ADD COLUMN wa_message_id TEXT`)

	// Padroniza e limpa JIDs antigos
	_, _ = DB.Exec(`UPDATE chat_messages SET chat_jid = SUBSTR(chat_jid, 1, INSTR(chat_jid, ':') - 1) || '@s.whatsapp.net' WHERE chat_jid LIKE '%:%'`)
	_, _ = DB.Exec(`UPDATE chat_messages SET chat_jid = REPLACE(chat_jid, '@c.us', '@s.whatsapp.net') WHERE chat_jid LIKE '%@c.us'`)
	_, _ = DB.Exec(`DELETE FROM chat_messages WHERE chat_jid LIKE '%@g.us%' OR chat_jid LIKE '%g.us%' OR chat_jid LIKE '120363%'`)

	// Limpa usuário padrão legado caso ainda exista com a senha padrão
	var defaultAdminPass string
	err = DB.QueryRow("SELECT password FROM users WHERE username = 'admin'").Scan(&defaultAdminPass)
	if err == nil {
		if defaultAdminPass == "admin123" || defaultAdminPass == "admin" || CheckPasswordHash("admin123", defaultAdminPass) || CheckPasswordHash("admin", defaultAdminPass) {
			_, _ = DB.Exec("DELETE FROM users WHERE username = 'admin'")
			fmt.Println("🧹 Usuário padrão legado 'admin' removido para permitir a configuração de Primeiro Acesso segura.")
		}
	}

	fmt.Println("✅ Banco de dados inicializado com sucesso.")
	iniciarRotinaLimpezaMidias()
	return nil
}

// HasAdminUser verifica se já existe ao menos um administrador ativo cadastrado no sistema
func HasAdminUser() bool {
	if DB == nil {
		return false
	}
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin' AND enabled = 1").Scan(&count)
	if err != nil {
		return false
	}
	return count > 0
}

func iniciarRotinaLimpezaMidias() {
	go func() {
		executarLimpezaMidias()
		ticker := time.NewTicker(6 * time.Hour)
		for range ticker.C {
			executarLimpezaMidias()
		}
	}()
}

func executarLimpezaMidias() {
	if DB != nil {
		res, err := DB.Exec(`
			UPDATE chat_messages 
			SET media_url = '' 
			WHERE timestamp < datetime('now', '-7 days') 
			  AND media_url LIKE 'data:%'
		`)
		if err != nil {
			fmt.Printf("🚨 Erro ao executar rotina de limpeza de mídias antigas no banco: %v\n", err)
		} else {
			rows, _ := res.RowsAffected()
			if rows > 0 {
				fmt.Printf("🧹 [LIMPEZA BANCO] %d mídias antigas (+7 dias) foram limpas do banco de dados!\n", rows)
				_, _ = DB.Exec("VACUUM")
			}
		}
	}

	uploadDir := filepath.Join(".", "static", "uploads")
	entries, errRead := os.ReadDir(uploadDir)
	if errRead == nil {
		limite7Dias := time.Now().Add(-7 * 24 * time.Hour)
		removidosCount := 0

		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == ".gitkeep" || entry.Name() == ".gitignore" {
				continue
			}

			filePath := filepath.Join(uploadDir, entry.Name())
			info, errInfo := entry.Info()
			if errInfo == nil && info.ModTime().Before(limite7Dias) {
				if errRemove := os.Remove(filePath); errRemove == nil {
					removidosCount++
				}
			}
		}

		if removidosCount > 0 {
			fmt.Printf("🧹 [LIMPEZA DISCO] %d arquivos temporários com mais de 7 dias foram excluídos da pasta uploads do servidor!\n", removidosCount)
		}
	}
}
