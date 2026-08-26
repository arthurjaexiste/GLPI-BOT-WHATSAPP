<p align="center">
  <img src="static/img/logo.png" alt="GLPI-BOT Logo" width="150" height="150">
</p>

# 🤖 GLPI-BOT: WhatsApp & Web Admin Panel

Um ecossistema robusto de alta performance desenvolvido em **Go (Golang)** projetado para conectar o **WhatsApp** diretamente ao sistema de chamados **GLPI**. Através de uma interface web administrativa moderna, responsiva e elegante, os administradores de TI e equipes de suporte podem estruturar árvores de conversação dinâmicas, gerenciar atendimentos humanos ao vivo (transbordo), controlar filas de espera com **RBAC (Administrador vs Operador)**, configurar parâmetros de conexão GLPI e acompanhar logs operacionais em tempo real.

---

## 🚀 Arquitetura Geral & Estrutura de Pastas

O projeto segue a estrutura padrão recomendada para aplicações Go modernas, separando o ponto de entrada (`cmd`) da lógica interna reutilizável (`internal`) e das interfaces de frontend (`web` / `static`):

```
GLPI-BOT/
├── cmd/
│   └── bot/
│       └── main.go                 # Ponto de entrada do bot, redirecionamento de logs e inicialização dos serviços
├── internal/
│   ├── config/
│   │   └── config.go               # Definição do schema de configurações, migração automática e valores padrões
│   ├── glpi/
│   │   └── glpi.go                 # Integração direta com a API Rest do GLPI (Abertura de chamados, envio de mídias e busca de usuários)
│   ├── state/
│   │   └── state.go                # Máquina de estados thread-safe com ActiveLiveChats map para cada número no WhatsApp
│   └── whatsapp/
│       ├── flow.go                 # Lógica de árvore de fluxo de navegação e enquetes interativas (WhatsApp Polls)
│       ├── handler.go              # Ouvinte de mensagens do whatsmeow, tratando reconexões, recebimento de mídias e ausência
│       ├── livechat.go             # Fila de atendimento humano ao vivo, transbordo e controle de atendentes ativos
│       ├── polls.go                # Utilitários para criação e processamento de votos/interações de enquetes
│       ├── smtp.go                 # Verificador de saúde da conexão que envia e-mails em caso de queda do bot
│       ├── ticket.go               # Fluxo passo a passo de coleta de dados de ticket (título, descrição, fotos e documentos)
│       ├── utils.go                # Funções utilitárias, citação de respostas (extrairInfoCitada), sanitização de JIDs e mídias
│       └── web.go                  # Painel Web administrativo rodando em servidor HTTP nativo com endpoints REST JSON e RBAC
├── static/                         # Ativos estáticos do painel administrativo (CSS, JS, Imagens, Ícones PWA)
├── web/                            # Templates HTML das páginas administrativas do painel
├── Dockerfile                      # Dockerfile otimizado para build multi-stage gerando uma imagem final ultra-leve
├── docker-compose.yml              # Arquivo de orquestração local para execução do banco e do bot
├── go.mod                          # Módulo Go contendo todas as dependências declaradas (whatsmeow, SQLite)
└── README.md                       # Documentação mestre do projeto
```

---

## 🌟 Funcionalidades Detalhadas

### 1. 💬 Fluxo Interativo com WhatsApp Polls (Enquetes)
Diferente dos bots legados baseados em digitação de números ("Digite 1 para suporte..."), o **GLPI-BOT** utiliza **enquetes nativas do WhatsApp (Polls)** para renderizar opções clicáveis de menu. Isso reduz a zero a taxa de erro do usuário e torna o fluxo extremamente ágil no celular.
- **Voltar ao Menu:** Permite configurar botões especiais para retornar ao nó pai da árvore de navegação de maneira imediata.
- **Visual Flow Builder:** Árvore de navegação customizável diretamente no painel administrativo.

### 2. 🎫 Coleta Inteligente de Tickets & Envio de Mídias
- **Abertura Passo a Passo:** O robô conduz a conversa coletando um título resumido e uma descrição detalhada do incidente.
- **Validação Automática de Solicitante:** Busca o número do remetente no banco de dados do GLPI e vincula o ticket ao colaborador correto automaticamente.
- **Suporte a Múltiplos Anexos:** Permite habilitar o envio opcional ou obrigatório de **imagens (prints)** e/ou **documentos (PDF, DOCX, XLSX)**. O bot faz o download da mídia do WhatsApp, converte-a e insere-a na aba de Documentos do GLPI, vinculando-a diretamente ao ticket gerado.

### 3. 👥 Live Chat Multi-Atendente com RBAC & Trava por Técnico
- **Controle de Acesso por Papel (RBAC)**:
  - **Operador (`role: operator`)**: Atendimento exclusivo por técnico. Quando um operador assume um cliente, o painel de outros operadores é bloqueado (`🔒 Atendimento exclusivo do técnico: Nome`), ocultando o botão `✓ Finalizar` e desativando a caixa de mensagens.
  - **Administrador (`role: admin`)**: Privilégios totais. Administradores podem visualizar o botão `✓ Finalizar`, responder, enviar mídias ou assumir qualquer atendimento humano em andamento.
- **Identificação com Bloco de Citação Nativo (`> 👨‍💻 *Nome:*`)**: As mensagens entregues ao WhatsApp do cliente chegam formatadas com o cabeçalho do técnico em bloco de citação nativo do WhatsApp, enquanto no painel web o histórico permanece limpo e elegante.
- **Citação de Mensagens (`extrairInfoCitada`)**: Suporte completo a respostas citadas no WhatsApp, exibindo os cartões de citação dentro dos balões do painel com o nome real do remetente ou técnico responsável.

### 4. 🧹 Gestão Otimizada de Mídias Zero-Disk
- As mídias recebidas e enviadas são convertidas e armazenadas em **Base64 Data URLs** diretamente no banco de dados SQLite.
- **Rotina Autônoma de Limpeza**: A cada 6 horas, um daemon em background limpa mídias Base64 com mais de 7 dias, exclui arquivos temporários de discos antigos e executa `VACUUM` no banco para otimizar espaço.

### 5. 📅 Controle de Expediente & Mensagem de Ausência
- O administrador define os dias úteis e a faixa de horário em que o suporte funciona. Fora do expediente, o chamado ainda pode ser aberto, mas o usuário recebe uma notificação configurável de ausência para alinhar expectativas de atendimento.

### 6. 🔔 Alertas para Equipes Técnicas & Blacklist
- **Notificações**: Assim que um ticket é gerado, o bot dispara um alerta no WhatsApp do técnico designado contendo o número do chamado e um link clicável direto para a página do ticket no GLPI.
- **Blacklist**: Números indesejados adicionados à blacklist são totalmente ignorados para preservar recursos.

---

## 🔑 Requisitos de Configuração no GLPI

A integração depende inteiramente do módulo de **API REST** do GLPI. Para configurar:

1. **Ative a API REST:**
   - Acesse o GLPI como Admin e vá em **Configurar > Geral > API**.
   - Habilite a opção **Habilitar API Rest**.
   - Ative **Habilitar login com credenciais de usuário** ou **external tokens**.
   - Adicione um novo cliente e obtenha o **App-Token** correspondente.

2. **Gere o User-Token:**
   - Acesse as preferências do usuário administrador (ou conta dedicada à automação do bot) no GLPI.
   - Vá na aba **Chaves de acesso remoto** (Remote Access Keys) e crie um **Token de API** (User-Token).

3. **Perfil de Permissões (Perfil Recomendado):**
   Garante que o usuário do bot tenha as seguintes permissões básicas ativas no GLPI:
   - **Usuários:** Leitura (Read) para localizar o telefone do solicitante.
   - **Chamados:** Criação e Atualização (Create/Update) para abrir e alimentar tickets.
   - **Documentos:** Criação (Create) para vincular imagens e anexos à base de conhecimento do ticket.

---

## 🚀 Instalação e Execução via Docker Compose

O bot foi empacotado em uma imagem estável hospedada e pode ser executado facilmente através de containers.

### Arquivo `docker-compose.yml`
Crie um diretório de trabalho no servidor e salve o arquivo com o seguinte conteúdo:

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

### Inicializando o Bot
Execute no terminal da pasta do arquivo:
```bash
docker compose up -d
```
Acompanhe os logs operacionais para verificar a correta inicialização dos servidores internos:
```bash
docker compose logs -f glpi-bot
```

---

## ⚙️ Primeiro Acesso & Configurações Iniciais

1. **Acessar o Painel:**
   Abra o navegador em `http://IP_DO_SERVIDOR:33090`. As credenciais padrão de primeiro acesso são:
   - **Usuário:** `admin`
   - **Senha:** `admin123`
   
   > [!IMPORTANT]
   > Lembre-se de alterar a senha administrativa padrão na aba **Configurações > Bloqueios & Senha** logo após o primeiro acesso para garantir a segurança da aplicação.

2. **Emparelhar WhatsApp:**
   Na aba **Status**, clique em **Conectar** para gerar o QR Code. Abra o seu WhatsApp no smartphone de suporte, clique em **Aparelhos conectados > Conectar um aparelho** e realize o escaneamento na tela.

3. **Cadastrar Credenciais do GLPI:**
   Acesse a aba **Configurações**, preencha o link absoluto da API Rest (ex: `https://meu-glpi/apirest.php`), o **App-Token** e o **User-Token** nos campos correspondentes e clique em **Salvar Configurações**.

---

## 🛠️ Resolução de Problemas Comuns

### 1. QR Code não carrega ou sessão desconecta frequentemente
- Certifique-se de que o container possui conexão ativa com a internet.
- Se o bot travar na tela de conexão, vá em **Configurações > Manutenção do Sistema** e utilize o botão **Reiniciar Bot** para limpar a engine local do whatsmeow e iniciar uma nova varredura de QR Code limpa.

### 2. Chamados criados como "Usuário Anônimo" ou GlpiUser não localizado
- Certifique-se de que o número do WhatsApp do solicitante está cadastrado no campo **Telefone** do seu respectivo usuário no GLPI no formato internacional sem o símbolo `+` (ex: `5511999999999`).
- O Perfil (Profile) da conta que gerou o `User-Token` deve ter permissão para ler a lista de usuários no GLPI para executar buscas completas.

### 3. Falha de upload de arquivos anexados
- O GLPI limita o tamanho padrão de uploads nas configurações de sistema php (`upload_max_filesize` e `post_max_size`). Certifique-se de que os limites do seu servidor GLPI toleram o envio de fotos ou documentos maiores.
- Valide se o perfil do usuário do bot tem acesso para escrever na pasta física de armazenamento de documentos no servidor onde o GLPI está hospedado.
