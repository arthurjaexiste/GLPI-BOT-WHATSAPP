package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bot-glpi/internal/config"
	"bot-glpi/internal/whatsapp"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	_ "modernc.org/sqlite"
)

func eventHandler(client *whatsmeow.Client) func(interface{}) {
	return func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			// Repassa a mensagem recebida para o seu roteador (handler.go)
			whatsapp.HandleMessage(client, evt)

		case *events.QR:
			// Salva o QR Code na variável global para o web.go ler
			whatsapp.CurrentQR = v.Codes[0]
			fmt.Println("QR Code gerado! Abra o painel web para escanear.")

		case *events.Connected:
			// Atualiza o status e limpa o QR Code apenas se estiver logado
			if client.IsLoggedIn() {
				whatsapp.IsConnected = true
				whatsapp.CurrentQR = ""
				fmt.Println("✅ Bot conectado ao WhatsApp com sucesso!")
			}

		case *events.Disconnected:
			whatsapp.IsConnected = false
			fmt.Println("❌ Bot desconectado do WhatsApp (queda de rede). Tentando reconexão automática...")

		case *events.LoggedOut:
			whatsapp.IsConnected = false
			whatsapp.CurrentQR = ""
			fmt.Println("❌ O bot foi deslogado do WhatsApp pelo celular.")
			
			go func() {
				// Aguarda a desconexão completa e inicia o pareamento com QR Code
				time.Sleep(2 * time.Second)
				fmt.Println("🔄 Inicializando novo canal de QR Code para re-pareamento...")
				qrChan, err := client.GetQRChannel(context.Background())
				if err != nil {
					fmt.Printf("🚨 Erro ao obter canal de QR Code: %v\n", err)
					return
				}
				err = client.Connect()
				if err != nil {
					fmt.Printf("🚨 Erro ao conectar para pareamento: %v\n", err)
				}
				for evt := range qrChan {
					if evt.Event == "code" {
						whatsapp.CurrentQR = evt.Code
						whatsapp.IsConnected = false
						fmt.Println("⚠️  NOVO QR CODE GERADO. VEJA NO PAINEL WEB OU ESCANEIE.")
					} else if evt.Event == "success" {
						whatsapp.IsConnected = true
						whatsapp.CurrentQR = ""
						fmt.Println("✅ Bot re-conectado ao WhatsApp com sucesso!")
					}
				}
			}()
		}
	}
}

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
	
	// 🟢 CORREÇÃO 1: Passando context.Background() como primeiro argumento
	container, err := sqlstore.New(context.Background(), "sqlite", "file:db/session.db?_pragma=foreign_keys(1)", nil)
	if err != nil {
		panic(err)
	}

	// 🟢 CORREÇÃO 2: Passando context.Background() como argumento
	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		panic(err)
	}

	// 3. Cria o cliente do WhatsApp
	client := whatsmeow.NewClient(deviceStore, nil)
	whatsapp.GlobalClient = client
	client.AddEventHandler(eventHandler(client))

	// 4. Inicia a conexão
	if client.Store.ID == nil {
		// Sem sessão salva: vai pedir QR Code
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		go func() {
			for evt := range qrChan {
				if evt.Event == "code" {
					whatsapp.CurrentQR = evt.Code
					whatsapp.IsConnected = false
					fmt.Println("⚠️  NOVO QR CODE GERADO. VEJA NO PAINEL WEB OU ESCANEIE.")
				} else if evt.Event == "success" {
					whatsapp.IsConnected = true
					whatsapp.CurrentQR = ""
					fmt.Println("✅ Bot conectado ao WhatsApp via QR Code!")
				}
			}
		}()
	} else {
		// Já possui sessão salva: conecta direto
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	// 5. Mantém o bot rodando até você apertar CTRL+C no terminal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	// Desconecta graciosamente ao desligar
	client.Disconnect()
}