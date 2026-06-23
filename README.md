# WhatsApp Bot Pro — Integração GLPI

<img src="img/ghopper.png" alt="Gopher GLPI" width="160" align="right">

Bot de WhatsApp profissional desenvolvido em **Go**, integrado nativamente ao **GLPI via REST API**. Realiza triagem automática de chamados de TI por enquetes nativas do WhatsApp e disponibiliza um **Painel Web** para configuração dinâmica do fluxo de atendimento.

<br clear="both">

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Docker-Supported-2496ED?style=for-the-badge&logo=docker&logoColor=white" alt="Docker">
  <img src="https://img.shields.io/badge/GLPI-REST%20API-9A0B0B?style=for-the-badge" alt="GLPI">
  <img src="https://img.shields.io/badge/GHCR-Published-24292E?style=for-the-badge&logo=github&logoColor=white" alt="GHCR">
</p>

---

## 🏗️ Arquitetura do Sistema

```mermaid
flowchart TD
    subgraph A["Fluxo de Atendimento - WhatsApp"]
        U["Usuário WhatsApp"] --> WA["Rede WhatsApp"]
        WA --> WM["Whatsmeow Client - Go"]
        WM --> Core["Core Handler - Go"]
        Core --> State[("Estado em RAM")]
        Core --> GLPI["GLPI REST API"]
        GLPI --> Core
        State --> Core
        WM --> U
    end

    subgraph B["Fluxo de Gerenciamento - Painel Web"]
        Admin["Administrador"] --> HTTP["Servidor Web Go - Porta 33090"]
        HTTP --> DB[("Pasta /db")]
        DB --> DB1["session.db"]
        DB --> DB2["web.db"]
        DB --> DB3["config.json"]
        DB --> DB4["flow.json"]
    end

    DB3 -. leitura em tempo real .-> Core
    DB4 -. leitura em tempo real .-> Core
```

- **Fluxo de Atendimento**: O usuário interage pelo WhatsApp. O `Core Handler` em Go processa os eventos, mantém o estado em memória e se comunica com o GLPI para registrar chamados.
- **Fluxo de Gerenciamento**: O administrador acessa o painel web na porta `33090`. Configurações gravadas no diretório `/db` são aplicadas dinamicamente, sem reiniciar o container.

---

## 💬 Fluxo de Mensagens do Bot

```mermaid
sequenceDiagram
    actor U as Usuário
    participant B as Bot WhatsApp
    participant G as GLPI API

    U->>B: Envia qualquer mensagem
    B->>U: Saudacao + Enquete de Categoria

    U->>B: Vota na categoria
    B->>U: Enquete de Subcategoria

    U->>B: Vota na subcategoria
    B->>U: Solicita descricao do problema

    U->>B: Descreve o problema
    B->>U: Solicita nome completo

    U->>B: Informa o nome
    B->>G: Busca usuario por telefone

    alt Usuario encontrado no GLPI
        G-->>B: Retorna dados do usuario
    else Usuario nao encontrado
        B->>G: Cria usuario Visitante temporario
    end

    B->>G: Abre chamado com titulo e descricao
    G-->>B: Retorna ID do chamado criado
    B->>U: Confirmacao com numero do chamado

    opt Usuario envia anexo
        U->>B: Envia foto ou documento
        B->>G: Upload do arquivo vinculado ao chamado
        B->>U: Confirma recebimento do anexo
    end
```

---

## 🚀 Como Usar — Passo a Passo

<p align="center">
  <img src="img/ghopperzap.png" alt="GhopperZap Bot" width="280">
</p>

### Pré-requisitos

- Docker e Docker Compose instalados no servidor
- Instância do GLPI acessível com a REST API habilitada

---

### Passo 1 — Criar a pasta e o `docker-compose.yml`

```bash
mkdir glpi-bot && cd glpi-bot
```

Crie o arquivo `docker-compose.yml`:

```yaml
services:
  bot:
    image: ghcr.io/arthurjaexiste/glpi-bot:latest
    container_name: glpi-bot
    restart: unless-stopped
    ports:
      - "33090:33090"
    volumes:
      - ./db:/app/db
```

---

### Passo 2 — Iniciar o bot

```bash
docker compose up -d
```

O Docker baixará automaticamente a imagem pré-compilada do GHCR.

---

### Passo 3 — Acessar o Painel Web

Abra no navegador: **`http://localhost:33090`** (ou o IP do servidor).

Credenciais padrão:
- **Usuário:** `admin`
- **Senha:** `admin123`

---

### Passo 4 — Conectar o WhatsApp

Na aba **Status**, escaneie o QR Code com o WhatsApp:

> **WhatsApp → Configurações → Dispositivos conectados → Conectar um dispositivo**

---

### Passo 5 — Configurar a integração com o GLPI

Na aba **Configurações**, preencha os campos e clique em **Salvar**:

| Campo | Descrição |
|---|---|
| `URL da API GLPI` | Ex: `https://glpi.empresa.com/apirest.php` |
| `App Token` | Token de aplicação gerado no GLPI |
| `User Token` | Token do usuário operador do bot |
| `Nome da Empresa` | Exibido nas mensagens automáticas |
| `Telefone de Alerta` | Número para notificações críticas |
| `Atendentes` | Lista de técnicos separados por vírgula |

As configurações entram em vigor imediatamente, sem reiniciar o container.

---

### Passo 6 — Personalizar o Fluxo de Atendimento

Na aba **Fluxo**, use o construtor visual para adicionar, renomear ou remover categorias. As alterações são aplicadas em tempo real.

---

### Passo 7 — Personalizar as Mensagens

Na aba **Mensagens**, edite os textos com as variáveis disponíveis:

| Variável | Substituído por |
|---|---|
| `{saudacao}` | Bom dia / Boa tarde / Boa noite |
| `{empresa}` | Nome da empresa configurado |
| `{ticket_id}` | Número do chamado aberto no GLPI |
| `{nome}` | Nome informado pelo usuário |

---

## ⚡ Deploy Automático via GitHub Actions

O projeto possui o workflow `.github/workflows/docker-publish.yml`. A cada push na branch `main`, uma imagem Docker é compilada e publicada automaticamente no **GitHub Container Registry (GHCR)**.

---

## 🛠️ Tecnologias Utilizadas

| Tecnologia | Biblioteca | Função no Projeto |
| :--- | :--- | :--- |
| **Go (Golang)** | `golang.org` | Linguagem principal. Alta performance, baixo consumo de memória e binário estático único. |
| **Whatsmeow** | `go.mau.fi/whatsmeow` | Protocolo de conexão Multi-Device criptografada com o WhatsApp. |
| **SQLite (Pure Go)** | `modernc.org/sqlite` | Banco embarcado para sessões WhatsApp e credenciais do painel. Sem CGO. |
| **Tailwind CSS** | `tailwindcss.com` CDN | Interface visual moderna e responsiva do Painel Web. |
| **HTML5 + Vanilla JS** | Nativo | Telas administrativas e construtor de árvore de decisão. |
| **GLPI REST API** | GLPI | Busca de usuários, abertura de chamados e upload de anexos. |
| **Docker + Compose** | `docker.com` | Containerização e distribuição da aplicação. |
| **GitHub Actions** | CI/CD | Build e publicação automática no GHCR a cada push na `main`. |
| **x/text/unicode** | `golang.org/x/text` | Normalização NFC/NFD para compatibilidade de enquetes em iOS/macOS. |

---

## 🎯 Funcionalidades

### 1. Triagem por WhatsApp Polls
O bot guia o usuário pela árvore de decisão usando **enquetes nativas do WhatsApp**. O usuário apenas clica na opção, eliminando erros de digitação.

### 2. Abertura Completa de Chamados no GLPI
- **Associação Automática**: Busca o remetente no GLPI por telefone. Se não existir, cria um usuário Visitante temporário.
- **Anexos**: Fotos, PDFs, DOCX e XLSX são vinculados diretamente ao chamado.

### 3. Compatibilidade iOS/macOS
Normalização Unicode (NFC/NFD) em tempo real evita que respostas de iPhones e Macs com caracteres acentuados decompostos sejam ignoradas.

### 4. Bloqueio em Chamados Finalizados
Chamados **Solucionados** ou **Fechados** bloqueiam novas interações e orientam o usuário a abrir um novo chamado.

### 5. Painel Administrativo Web

| Aba | Função |
|---|---|
| **Status** | Conexão em tempo real com WhatsApp e QR Code dinâmico |
| **Fluxo** | Editor visual da árvore de decisão |
| **Configurações** | Chaves GLPI, telefone de alerta e lista negra |
| **Mensagens** | Personalização de textos com variáveis dinâmicas |

---

## 🛡️ Resiliência e Segurança

- **Mutex (`sync.RWMutex`)**: Protege leituras e escritas concorrentes no estado em memória, evitando race conditions.
- **Graceful Shutdown**: Sinais `SIGTERM`/`SIGINT` encerram conexões e sessões de forma limpa.
- **Auto-Reconnect**: O Whatsmeow reconecta automaticamente em caso de instabilidade de rede.
