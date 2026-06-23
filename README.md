# 🤖 GLPI-BOT (WhatsApp & Web Panel)

Um bot de WhatsApp inteligente e de alta performance desenvolvido em **Go**, projetado para integrar fluxos de conversação dinâmicos diretamente com o **GLPI**. Gerencie menus, submenus, formulários de abertura de chamado e anexos através de um painel web moderno.

---

## 🛠️ Tecnologias Utilizadas

- **Backend:** [Go (Golang)](https://golang.org) - Eficiente, seguro e ultrarrápido.
- **Protocolo WhatsApp:** [Whatsmeow](https://github.com/tulir/whatsmeow) - Conexão estável e segura via WhatsApp Multi-Device.
- **Banco de Dados:** SQLite (puro Go, sem dependência CGO) para persistência de sessões.
- **Integração GLPI:** REST API nativa do GLPI (Abertura de chamados, busca de usuários e upload de anexos).
- **Interface Web:** HTML5, Vanilla JavaScript e TailwindCSS (via CDN) para o painel administrativo.
- **Containerização:** Docker e Docker Compose.

---

## 💬 Fluxo do Bot no WhatsApp

1. **Saudação e Identificação:** O usuário envia uma mensagem e o bot verifica se ele já está cadastrado no GLPI.
2. **Menu Dinâmico (WhatsApp Polls):** O bot exibe o menu inicial configurado no painel web em forma de enquetes do WhatsApp.
3. **Navegação (Submenus):** O usuário navega por submenus (com suporte a botão de voltar opcional).
4. **Coleta de Informações (Chamado):** Ao selecionar a opção de abertura de chamado:
   - Coleta um **título curto**.
   - Coleta a **descrição detalhada** do problema.
   - Solicita **fotos/prints** e/ou **documentos** (se habilitados para a opção no painel).
5. **Criação do Ticket:** O chamado é aberto no GLPI em nome do solicitante, vinculando a descrição e todos os anexos enviados. O usuário recebe a confirmação com o número do ticket.
6. **Acompanhamento de Chamado:** Permite buscar o status de chamados existentes e adicionar novas interações de forma controlada.

---

## 🚀 Como Executar o Bot

### 1. Criar o arquivo `docker-compose.yml`
No diretório do projeto, certifique-se de que possui o seguinte arquivo `docker-compose.yml`:

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

### 2. Iniciar o Container
Para baixar a imagem pronta do GHCR e iniciar o serviço, execute:

```bash
docker compose up -d
```

### 3. Acessar o Painel Web
Abra o navegador no endereço: **`http://localhost:33090`** (ou IP do seu servidor).

- **Usuário padrão:** `admin`
- **Senha padrão:** `admin123`

---

## ⚙️ Configuração Inicial no Painel

1. **Status:** Escaneie o QR Code exibido com o seu aplicativo do WhatsApp (Dispositivos Conectados) para iniciar o bot.
2. **Configurações:** Configure a URL da API do seu GLPI, tokens de acesso (`App Token` e `User Token`) e parametrize a lista de técnicos.
3. **Fluxo do Bot:** Crie a sua árvore de decisões usando o construtor visual de menus, submenus e formulários de chamado com ou sem anexos.
4. **Mensagens:** Personalize as respostas automáticas enviadas pelo bot no WhatsApp usando as variáveis disponíveis (`{nome}`, `{ticket_id}`, etc).
