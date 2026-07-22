// ============================================================================
// ARQUIVO: main.go
// Descrição: Implementação Go (backend) para o ecossistema GLPI-BOT.
// ============================================================================

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"bot-glpi/internal/config"
	"bot-glpi/internal/whatsapp"

	"go.mau.fi/whatsmeow/store/sqlstore"
	_ "modernc.org/sqlite"
)

// setupLogRedirection duplica toda a saída padrão (stdout/stderr) para um arquivo de log
// enquanto mantém a exibição no terminal, permitindo diagnóstico remoto pelo painel web.

// Função setupLogRedirection executa a regra de negócio/rotina correspondente
func setupLogRedirection() {
	_ = os.MkdirAll("db", 0777)

	f, err := os.OpenFile("db/bot.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("🚨 Erro ao criar arquivo de log: %v\n", err)
		return
	}

	r, w, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = w
	os.Stderr = w

	go func() {
		buf := make([]byte, 2048)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				_, _ = origStdout.Write(buf[:n])
				_, _ = f.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()
}


// Função main executa a regra de negócio/rotina correspondente
func main() {
	setupLogRedirection()

	// Detecta se o bot foi reiniciado pelo painel web
	restartedFile := "db/.restarted"
	if _, err := os.Stat(restartedFile); err == nil {
		fmt.Println("🔄 O bot foi REINICIADO com sucesso pelo Painel Web!")
		os.Remove(restartedFile)
	} else {
		fmt.Println("🚀 O bot foi INICIADO com sucesso!")
	}

	// Inicializa as configurações globais e o fluxo de conversa
	config.InitConfig()
	whatsapp.InitFlow()

	// Inicia o servidor do Painel Web em paralelo (porta 33090)
	go whatsapp.StartWebServer()

	// Prepara o banco de dados da sessão do WhatsApp (SQLite)
	os.MkdirAll("db", 0777)
	container, err := sqlstore.New(context.Background(), "sqlite", "file:db/session.db?_pragma=foreign_keys(1)", nil)
	if err != nil {
		panic(err)
	}

	whatsapp.GlobalContainer = container
	if err = whatsapp.StartWhatsApp(context.Background()); err != nil {
		panic(err)
	}

	// Inicia o monitor de conexão SMTP em segundo plano
	go whatsapp.StartSMTPChecker(context.Background())

	// Mantém o bot rodando até receber SIGINT ou SIGTERM
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// Desconecta graciosamente ao desligar
	if whatsapp.GlobalClient != nil {
		whatsapp.GlobalClient.Disconnect()
	}
}