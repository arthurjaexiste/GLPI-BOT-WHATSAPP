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

func main() {
	setupLogRedirection()

	// Verifica se foi reiniciado pelo painel web
	restartedFile := "db/.restarted"
	if _, err := os.Stat(restartedFile); err == nil {
		fmt.Println("🔄 O bot foi REINICIADO com sucesso pelo Painel Web!")
		os.Remove(restartedFile)
	} else {
		fmt.Println("🚀 O bot foi INICIADO com sucesso!")
	}

	// 🟢 Inicializa as configurações globais do bot
	config.InitConfig()

	// 🟢 Inicializa o fluxo de conversa dinâmico
	whatsapp.InitFlow()

		// 1. Inicia o servidor do Painel Web em paralelo (porta 33090)
	go whatsapp.StartWebServer()

	// 2. Prepara o banco de dados da sessão do WhatsApp (SQLite)
	os.MkdirAll("db", 0777)
	
	container, err := sqlstore.New(context.Background(), "sqlite", "file:db/session.db?_pragma=foreign_keys(1)", nil)
	if err != nil {
		panic(err)
	}

	whatsapp.GlobalContainer = container
	err = whatsapp.StartWhatsApp(context.Background())
	if err != nil {
		panic(err)
	}

	// 5. Mantém o bot rodando até você apertar CTRL+C no terminal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// Desconecta graciosamente ao desligar
	if whatsapp.GlobalClient != nil {
		whatsapp.GlobalClient.Disconnect()
	}
}