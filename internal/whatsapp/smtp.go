package whatsapp

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"bot-glpi/internal/config"
)

// ─── Estado do monitor SMTP ───────────────────────────────────────────────────

var (
	smtpAlertSent bool
	offlineSince  time.Time
)

// ─── Monitor de conexão ───────────────────────────────────────────────────────

// StartSMTPChecker inicia o monitor de conexão do bot em segundo plano.
// A cada 30 segundos verifica se o bot está online; após 10 minutos offline,
// envia um e-mail de alerta (se o SMTP estiver configurado).
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

// checkConnectionAndSendEmail avalia o estado da conexão e dispara o alerta
// se o bot estiver offline por mais de 10 minutos.
func checkConnectionAndSendEmail() {
	ClientMu.Lock()
	connected := IsConnected
	ClientMu.Unlock()

	if connected {
		smtpAlertSent = false
		offlineSince = time.Time{}
		return
	}

	if offlineSince.IsZero() {
		offlineSince = time.Now()
		fmt.Printf("⚠️  [SMTP MONITOR] Bot detectado offline em %s. Iniciando contagem de 10 minutos...\n", offlineSince.Format("15:04:05"))
		return
	}

	durationOffline := time.Since(offlineSince)
	if durationOffline >= 10*time.Minute && !smtpAlertSent {
		cfg := config.GetConfig()
		if !cfg.SMTPEnabled || cfg.SMTPHost == "" || cfg.SMTPUsername == "" {
			return
		}

		fmt.Printf("🚨 [SMTP MONITOR] Bot offline há %v. Enviando e-mail de alerta...\n", durationOffline)

		if err := sendSMTPEmail(cfg, smtpAlertPayload(cfg)); err != nil {
			fmt.Printf("🚨 [SMTP MONITOR] Falha ao enviar e-mail de alerta: %v\n", err)
		} else {
			fmt.Println("📧 [SMTP MONITOR] E-mail de alerta enviado com sucesso!")
			smtpAlertSent = true
		}
	}
}

// ─── Envio de e-mails ─────────────────────────────────────────────────────────

// emailPayload agrupa os dados necessários para compor e enviar um e-mail.
type emailPayload struct {
	subject string
	body    string
}

// sendSMTPEmail envia um e-mail HTML usando as configurações SMTP do bot.
func sendSMTPEmail(cfg config.Config, payload emailPayload) error {
	host := smtpHost(cfg.SMTPHost)
	auth := smtp.PlainAuth("", cfg.SMTPUsername, cfg.SMTPPassword, host)
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\r\n\r\n"
	msg := []byte(payload.subject + mime + payload.body)
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	return smtp.SendMail(addr, auth, cfg.SMTPSender, []string{cfg.SMTPReceiver}, msg)
}

// smtpHost extrai apenas o hostname de uma string que pode conter "host:porta".
func smtpHost(hostPort string) string {
	if strings.Contains(hostPort, ":") {
		return strings.Split(hostPort, ":")[0]
	}
	return hostPort
}

// smtpAlertPayload monta o e-mail de alerta de desconexão do bot.
func smtpAlertPayload(cfg config.Config) emailPayload {
	subject := fmt.Sprintf("Subject: 🚨 ALERTA: Bot GLPI do WhatsApp Desconectado! (%s)\r\n", cfg.CompanyName)
	body := fmt.Sprintf(`
		<html>
		<body style="font-family: Arial, sans-serif; color: #333; line-height: 1.6;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #ddd; border-radius: 8px; background-color: #f9f9f9;">
				<h2 style="color: #d9534f; margin-top: 0;">🚨 Alerta de Desconexão do Bot</h2>
				<p>Olá Administrador,</p>
				<p>O robô do WhatsApp da empresa <strong>%s</strong> perdeu a conexão e o QR Code gerado <strong>não foi escaneado nos últimos 10 minutos</strong>.</p>
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
					Configuração SMTP: %s:%d
				</p>
			</div>
		</body>
		</html>
	`, cfg.CompanyName, cfg.SMTPHost, cfg.SMTPPort)

	return emailPayload{subject: subject, body: body}
}

// smtpTestPayload monta o e-mail de teste de configuração SMTP.
func smtpTestPayload(cfg config.Config) emailPayload {
	subject := fmt.Sprintf("Subject: 🧪 TESTE: Envio SMTP do Bot GLPI (%s)\r\n", cfg.CompanyName)
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
	`, cfg.SMTPHost, cfg.SMTPPort)

	return emailPayload{subject: subject, body: body}
}

// ─── API pública ──────────────────────────────────────────────────────────────

// SendSMTPTestEmail envia um e-mail de teste para validar as configurações SMTP.
func SendSMTPTestEmail(cfg config.Config) error {
	return sendSMTPEmail(cfg, smtpTestPayload(cfg))
}
