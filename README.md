# GLPI-BOT

Sistema de automação de atendimento via WhatsApp integrado à API REST do GLPI Helpdesk. Desenvolvido em Go utilizando a biblioteca Whatsmeow, o sistema oferece navegação interativa via enquetes nativas, abertura e consulta de chamados, suporte ao vivo (live chat) com controle de acesso baseado em funções (RBAC), e painel web administrativo embarcado.

## Pré-requisitos

- Go 1.20 ou superior
- CGO / SQLite3
- Instância do GLPI com a API REST habilitada (App-Token e User-Token configurados)

## Compilação e Execução

### Execução Direta (Local)

1. Compile o executável a partir do diretório raiz:
   ```bash
   go build -o bot ./cmd/bot
   ```

2. Execute o binário gerado:
   ```bash
   ./bot
   ```

3. Acesse o painel web administrativo no navegador:
   `http://localhost:33090` (Credenciais iniciais: `admin` / `admin123`)

### Execução via Docker Compose

```yaml
version: '3.8'

services:
  glpi-bot:
    image: ghcr.io/arthurjaexiste/glpi-bot:latest
    container_name: glpi-bot
    restart: unless-stopped
    ports:
      - "33090:33090"
    volumes:
      - ./db:/app/db
```

```bash
docker compose up -d
```

## Variáveis de Configuração (.env)

| Variável | Descrição | Valor Padrão |
| :--- | :--- | :--- |
| `GLPI_API_URL` | URL base da API REST do GLPI | `http://localhost/glpi/apirest.php` |
| `GLPI_APP_TOKEN` | Token da aplicação configurado no GLPI | - |
| `GLPI_USER_TOKEN` | Token do usuário de serviço no GLPI | - |
| `COMPANY_NAME` | Nome da empresa para exibição no bot | `Suporte TI` |
| `TELEFONE_NOTIFICACAO` | Telefone do grupo/suporte para alertas | - |
| `SUPPORT_AGENTS` | Lista de nomes de atendentes separados por vírgula | - |
| `WORKING_HOURS_START` | Horário de início do expediente (HH:MM) | `08:00` |
| `WORKING_HOURS_END` | Horário de término do expediente (HH:MM) | `18:00` |
| `WORKING_DAYS` | Dias de expediente (1=Segunda a 5=Sexta) | `1,2,3,4,5` |
| `SMTP_ENABLED` | Habilita o envio de alertas de desconexão por e-mail | `false` |
