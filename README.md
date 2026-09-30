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
   `http://localhost:33090` (No primeiro acesso, a tela de configuração inicial será exibida para criar a sua conta de Administrador)

### Execução via Docker Compose

```yaml
version: '3.8'

services:
  glpi-bot:
    image: glpi-bot:latest
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

## Configurações pelo Painel Web

Todas as configurações do sistema são realizadas diretamente pela interface web administrativa (`http://localhost:33090`), sendo salvas e persistidas no diretório `db/` (`db/config.json`). Não é necessário criar nem utilizar arquivos `.env`.

Após criar o seu usuário Administrador no primeiro acesso e efetuar o login, acesse as abas do painel:

1. **Status**:
   - Clique em **Conectar** e escaneie o QR Code com o WhatsApp de atendimento.

2. **Configurações**:
   - **Identidade & Suporte**: Nome da empresa e telefone de notificação técnica.
   - **Conexão GLPI API**: URL da API REST (`https://seu-glpi/apirest.php`), **App-Token** e **User-Token**.
   - **Horário & Ausência**: Definição de horários de expediente, dias da semana e mensagem automática de ausência.
   - **Alerta E-mail (SMTP)**: Servidor e credenciais para alertas caso o bot desconecte.
   - **Gestão de Usuários**: Criação e controle de acesso para administradores e operadores.

3. **Mensagens & Fluxo**:
   - Personalize textos, variáveis dinâmicas e a árvore de navegação com enquetes interativas.

