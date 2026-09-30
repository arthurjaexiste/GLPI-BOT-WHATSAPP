package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"bot-glpi/internal/config"
	"bot-glpi/internal/database"
	"bot-glpi/internal/state"
	"bot-glpi/internal/web"
	"bot-glpi/internal/whatsapp"

	"go.mau.fi/whatsmeow/store/sqlstore"
	_ "modernc.org/sqlite"
)

func setupLogRedirection() {
	_ = os.MkdirAll("db", 0777)

	f, err := os.OpenFile("db/bot.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Printf("Erro ao criar arquivo de log: %v\n", err)
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

	restartedFile := "db/.restarted"
	if _, err := os.Stat(restartedFile); err == nil {
		fmt.Println("O bot foi reiniciado pelo Painel Web.")
		os.Remove(restartedFile)
	} else {
		fmt.Println("Serviço iniciado com sucesso.")
	}

	database.InitDB()
	config.InitConfig()
	state.LoadState()
	state.StartPersister()
	whatsapp.InitFlow()

	go web.StartWebServer()

	os.MkdirAll("db", 0777)
	container, err := sqlstore.New(context.Background(), "sqlite", "file:db/session.db?_pragma=foreign_keys(1)", nil)
	if err != nil {
		panic(err)
	}

	whatsapp.GlobalContainer = container
	if err = whatsapp.StartWhatsApp(context.Background()); err != nil {
		panic(err)
	}

	go whatsapp.StartSMTPChecker(context.Background())

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	if whatsapp.GlobalClient != nil {
		whatsapp.GlobalClient.Disconnect()
	}
}
