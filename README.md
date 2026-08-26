<p align="center">
  <img src="static/img/logo.png" alt="GLPI-BOT Logo" width="140" height="140">
</p>

<h1 align="center">🤖 GLPI-BOT | WhatsApp & Web Admin Panel</h1>

<p align="center">
  <b>Sistema Inteligente de Automação de Suporte e Live Chat para WhatsApp integrado ao GLPI Helpdesk.</b>
</p>

---

## 📌 Funcionalidades Principais

- 🤖 **Autoatendimento com Enquetes (Polls)**: Navegação ágil e sem erros através de botões interativos nativos do WhatsApp.
- 🎫 **Abertura de Chamados no GLPI**: Coleta automatizada de títulos, descrições e vínculo com o solicitante pelo telefone.
- 👥 **Live Chat Multi-Atendente com RBAC**:
  - **Operador (`operator`)**: Atendimento exclusivo com trava por técnico (`🔒 Atendimento exclusivo do técnico: Nome`).
  - **Administrador (`admin`)**: Acesso total para assumir, responder e encerrar qualquer conversa.
- 💬 **Citação Nativa no WhatsApp**: Mensagens entregues ao cliente formatadas com `> 👨‍💻 *Técnico:*` e suporte a respostas citadas no painel.
- 🧹 **Gestão Otimizada de Mídias Zero-Disk**: Mídias em Base64 Data URL no SQLite com limpeza automática a cada 6 horas (retenção de 7 dias e `VACUUM`).
- 📅 **Controle de Expediente & Ausência**: Horários de atendimento configuráveis com mensagem de retorno automático.

---

## 📂 Estrutura do Projeto

```text
GLPI-BOT/
├── cmd/bot/main.go       # Ponto de entrada e inicialização dos serviços
├── internal/
│   ├── config/           # Schema de configurações e persistência JSON
│   ├── glpi/             # Cliente API REST do GLPI (Chamados e Mídias)
│   ├── state/            # Estado global thread-safe por número (ActiveLiveChats map)
│   └── whatsapp/         # Lógica do robô, Whatsmeow, Flow Builder, LiveChat e Web Server
├── static/               # Estilos (CSS), scripts JS reativos e imagens
├── web/                  # Templates HTML das páginas do painel administrativo
├── Dockerfile            # Imagem Docker multi-stage otimizada
└── docker-compose.yml    # Orquestração simplificada do serviço
```

---

## 🖥️ Mapeamento do Painel Web

1. 📊 **Status & Conexão**: QR Code dinâmico do Whatsmeow e métricas em tempo real.
2. 🔀 **Fluxo do Bot**: Construtor visual de menus e enquetes clicáveis.
3. ⚙️ **Configurações**: Parâmetros de API do GLPI, expediente, mensagens de ausência e SMTP.
4. ✉️ **Mensagens**: Templates personalizáveis com variáveis `{nome}`, `{posicao}`, `{agente}`, `{ticket_id}`.
5. 💬 **Live Chat**: Chat ao vivo estilo WhatsApp Web com suporte a áudio, imagens, anexos e trava por técnico.
6. 👥 **Usuários & RBAC**: Gestão de acessos com permissões diferenciadas para Administradores e Operadores.
7. 📋 **Logs**: Console em tempo real para monitoramento e auditoria.

---

## 🔑 Configuração no GLPI

1. **Ativar API REST**: Em **Configurar > Geral > API**, habilite a API Rest e crie um **App-Token**.
2. **Gerar User-Token**: Nas preferências do usuário do bot no GLPI, gere a chave API (**User-Token**).
3. **Permissões Mínimas**: Garantir que o perfil do usuário tenha permissão de **Leitura** em Usuários e **Criar/Atualizar** em Chamados e Documentos.

---

## 🚀 Como Executar via Docker Compose

### 1. Crie o arquivo `docker-compose.yml`:
```yaml
version: "3.8"

services:
  glpi-bot:
    image: ghcr.io/arthurjaexiste/glpi-bot:latest
    container_name: glpi-bot
    restart: unless-stopped
    ports:
      - "33090:33090"
    volumes:
      - ./db:/app/db
      - ./config:/app/config
```

### 2. Inicie o container:
```bash
docker compose up -d
```

### 3. Primeiro Acesso:
Acesse `http://IP_DO_SERVIDOR:33090` no navegador. Credenciais padrão:
- **Usuário:** `admin`
- **Senha:** `admin123`

---

## 🛠️ Resolução de Problemas

- **QR Code não carrega**: Clique em **Reiniciar Bot** no painel de configurações para resetar a engine.
- **Chamado Anônimo**: Cadastre o telefone do colaborador no GLPI no formato internacional sem o `+` (ex: `5575999999999`).
- **Mídia não envia**: Verifique se o tamanho do arquivo respeita os limites de upload configurados no PHP do seu servidor GLPI.

---

<p align="center">
  Desenvolvido por <b>Arthur Salles</b> • Distribuído sob Licença <b>MIT</b>
</p>
