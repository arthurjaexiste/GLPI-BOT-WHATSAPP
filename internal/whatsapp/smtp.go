package whatsapp

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"bot-glpi/internal/config"
)

var (
	smtpAlertSent bool
	offlineSince  time.Time
)

func StartSMTPChecker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	fmt.Println("📧 Monitor de conexão SMTP inicializado em segundo plano.")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkConnectionAndSendEmail()
		}
	}
}

func checkConnectionAndSendEmail() {
	ClientMu.Lock()
	connected := IsConnected
	ClientMu.Unlock()

	if connected {
		// Bot online: reseta rastreadores
		smtpAlertSent = false
		offlineSince = time.Time{}
		return
	}

	// Bot offline
	if offlineSince.IsZero() {
		offlineSince = time.Now()
		fmt.Printf("⚠️  [SMTP MONITOR] Bot detectado offline em %s. Iniciando contagem de 10 minutos para alerta...\n", offlineSince.Format("15:04:05"))
		return
	}

	durationOffline := time.Since(offlineSince)
	if durationOffline >= 10*time.Minute && !smtpAlertSent {
		cfg := config.GetConfig()
		if !cfg.SMTPEnabled || cfg.SMTPHost == "" || cfg.SMTPUsername == "" {
			// SMTP desativado ou não configurado
			return
		}

		fmt.Printf("🚨 [SMTP MONITOR] Bot está offline há %v. Enviando e-mail de alerta...\n", durationOffline)
		err := sendSMTPAlertEmail(cfg)
		if err != nil {
			fmt.Printf("🚨 [SMTP MONITOR] Falha ao enviar e-mail de alerta SMTP: %v\n", err)
		} else {
			fmt.Println("📧 [SMTP MONITOR] E-mail de alerta de desconexão enviado com sucesso!")
			smtpAlertSent = true
		}
	}
}

func sendSMTPAlertEmail(cfg config.Config) error {
	// Configurações do SMTP
	// Resolve host do servidor para auth
	host := cfg.SMTPHost
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}
	auth := smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, host)

	// Assunto e corpo do email
	subject := fmt.Sprintf("Subject: 🚨 ALERTA: Bot GLPI do WhatsApp Desconectado! (%s)\r\n", cfg.CompanyName)
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\r\n\r\n"
	
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; color: #333; line-height: 1.6;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #ddd; border-radius: 8px; background-color: #f9f9f9;">
				<h2 style="color: #d9534f; margin-top: 0;">🚨 Alerta de Desconexão do Bot</h2>
				<p>Olá Administrador,</p>
				<p>O robô do WhatsApp da empresa <strong>%%s</strong> perdeu a conexão com a rede e o QR Code gerado <strong>não foi escaneado nos últimos 10 minutos</strong>.</p>
				<p style="background-color: #f2dede; padding: 15px; border-left: 5px solid #d9534f; border-radius: 4px; font-weight: bold; color: #a94442;">
					⚠️ O atendimento automático aos clientes está INDISPONÍVEL neste momento.
				</p>
				<p>Por favor, acesse o painel de administração imediatamente para visualizar e ler o novo QR Code:</p>
				<p style="text-align: center; margin: 30px 0;">
					<a href="http://localhost:33090" style="background-color: #0275d8; color: white; padding: 12px 24px; text-decoration: none; border-radius: 5px; font-weight: bold; display: inline-block; box-shadow: 0 4px 6px rgba(0,0,0,0.1);">
						🔗 Acessar Painel Web
					</a>
				</p>
				<hr style="border: 0; border-top: 1px solid #eee; margin: 20px 0;">
				<p style="font-size: 11px; color: #777;">
					Este é um e-mail automático gerado pelo sistema de monitoramento do GLPI-BOT.<br>
					Configuração SMTP: %%s:%%d
				</p>
			</div>
		</body>
		</html>
	`, cfg.CompanyName, cfg.SMTPHost, cfg.SMTPPort)

	msg := []byte(subject + mime + body)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	// Envia o e-mail
	err := smtp.SendMail(addr, auth, cfg.SMTPSender, []string{cfg.SMTPReceiver}, msg)
	return err
}

func SendSMTPTestEmail(cfg config.Config) error {
	host := cfg.SMTPHost
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}
	auth := smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, host)

	subject := fmt.Sprintf("Subject: 🧪 TESTE: Envio SMTP do Bot GLPI (%s)\r\n", cfg.CompanyName)
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\r\n\r\n"
	
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; color: #333; line-height: 1.6;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #ddd; border-radius: 8px; background-color: #f9f9f9;">
				<h2 style="color: #0275d8; margin-top: 0;">🧪 Teste de Conexão SMTP</h2>
				<p>Olá Administrador,</p>
				<p>Este é um e-mail de teste enviado a partir do seu <strong>GLPI-BOT</strong> para verificar as configurações de SMTP.</p>
				<p style="background-color: #dff0d8; padding: 15px; border-left: 5px solid #5cb85c; border-radius: 4px; font-weight: bold; color: #3c763d;">
					✅ Parabéns! Suas configurações de SMTP estão corretas e o envio está funcionando!
				</p>
				<hr style="border: 0; border-top: 1px solid #eee; margin: 20px 0;">
				<p style="font-size: 11px; color: #777;">
					Este é um e-mail de teste automático.<br>
					Configuração SMTP: %s:%d
				</p>
			</div>
		</body>
		</html>
	`, cfg.CompanyName, cfg.SMTPHost, cfg.SMTPPort)

	msg := []byte(subject + mime + body)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	err := smtp.SendMail(addr, auth, cfg.SMTPSender, []string{cfg.SMTPReceiver}, msg)
	return err
}
