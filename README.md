<p align="center">
  <img src="static/img/logo.png" alt="GLPI-BOT Logo" width="150" height="150">
</p>

# 🤖 GLPI-BOT: WhatsApp & Web Admin Panel

Um ecossistema robusto e de alta performance desenvolvido em **Go (Golang)** que conecta o **WhatsApp** diretamente ao seu sistema de chamados **GLPI**. Com uma interface administrativa moderna e intuitiva, o bot gerencia fluxos dinâmicos de atendimento, abre chamados automaticamente com suporte a anexos (fotos e documentos), e envia alertas aos técnicos com links diretos.

---

## 🌟 Funcionalidades Detalhadas do Bot

O **GLPI-BOT** oferece recursos completos para automatizar e otimizar o suporte de TI pelo WhatsApp:

### 1. 💬 Fluxo Interativo de Conversação (Visual Flow Builder)
- **Menu Principal e Submenus Ilimitados:** Estruture a árvore de decisão do bot através do painel. A navegação no WhatsApp é feita por **enquetes interativas (WhatsApp Polls)** nativas do aplicativo, eliminando erros de digitação.
- **Botão "Voltar":** Habilite opcionalmente botões de retorno nas enquetes para facilitar a navegação do usuário.
- **Acompanhamento de Tickets:** Permite ao usuário do WhatsApp consultar seus chamados abertos e o status de cada um no GLPI em tempo real.

### 2. 🎫 Coleta Inteligente e Abertura de Chamados (Integração GLPI)
- **Vínculo por Usuário:** O bot pesquisa o número de telefone no cadastro de usuários do GLPI para identificar o solicitante.
- **Coleta de Informações Passo a Passo:** Solicita ao usuário um título resumido e uma descrição detalhada do problema.
- **Upload Automático de Anexos:** Habilite se o bot deve pedir **fotos/prints** da tela e/ou **documentos** (PDF, Word, planilhas) após a descrição. Os arquivos são carregados diretamente na aba de Documentos do GLPI e vinculados ao ticket criado.
- **Confirmação com ID:** O usuário recebe a confirmação imediata da abertura do chamado com o número do ticket.

### 3. 📅 Controle de Expediente e Ausência (Filtro de Horário)
- **Configuração de Dias de Trabalho:** Marque no painel quais dias da semana a TI atende (Segunda a Domingo).
- **Faixa de Horário Útil:** Configure a hora de início e fim do atendimento (ex: `08:00` às `18:00`).
- **Mensagem de Ausência Automática:** Fora do expediente ou em dias não úteis, o bot continuará abrindo chamados, mas enviará um alerta amigável de ausência ("Nosso expediente é de Segunda a Sexta...").

### 4. 🔔 Notificação para a Equipe Técnica
- **Aviso no WhatsApp do Técnico:** Quando um usuário abre um chamado no WhatsApp, o bot envia um alerta direto para o telefone configurado para a equipe de suporte.
- **Link Direto Clicável:** O alerta contém o link direto para o chamado no painel do GLPI:
  ```
  📌 Ticket: #1245 http://seu-glpi/index.php?redirect=ticket_1245
  ```

### 5. 👥 Fila de Suporte Humano (Transbordo)
- **Direcionamento ao Técnico:** Uma das opções do fluxo pode ser "Falar com Suporte".
- **Fila de Atendimento:** O usuário entra em uma fila de espera temporária com controle de posição ("Sua posição na fila é 2º").
- **Técnicos Configurados:** Permite definir atendentes humanos para assumir o chat diretamente no WhatsApp.

### 6. 🛡️ Segurança, Alertas e Manutenção
- **Alerta de Queda por E-mail (SMTP):** Configuração de servidor de e-mail (SMTP) para alertar os gestores se a sessão do WhatsApp do bot for desconectada.
- **Blacklist (Lista de Bloqueio):** Cadastre números indesejados (como grupos ou números spam) para serem sumariamente ignorados pelo bot.
- **Logs em Tempo Real:** Console interativo no painel administrativo para visualizar o processamento e comportamento do robô linha por linha.
- **Reinício Remoto:** Botão para reiniciar o executável do bot remotamente através do painel.

---

## 🔑 Requisitos de Configuração no GLPI

Para integrar o bot com sucesso, você precisará configurar o acesso à API Rest do GLPI e garantir as permissões de usuário adequadas.

### 1. Habilitar a API Rest no GLPI
1. Acesse o GLPI com perfil administrador.
2. Vá em **Configurar > Geral > API**.
3. Ative a opção **Habilitar API Rest**.
4. Habilite **Habilitar login com credenciais de usuário** ou **Habilitar login com tokens externos**.
5. Clique em **Adicionar Token de API** (App-Token). Copie este token (você o usará na aba de configurações do bot).

### 2. Obter o User-Token (Token de Usuário)
1. Vá nas preferências do usuário que servirá para a integração (geralmente uma conta exclusiva do bot ou do administrador de TI).
2. Na aba **Chaves de Acesso Remoto**, gere um **Token de API** (User-Token). Copie esta chave.

### 3. Permissões Necessárias para o Perfil (Profile) no GLPI
O usuário associado ao `User-Token` deve ter um perfil atribuído com as seguintes permissões:
- **Usuários (Users):** Permissão de **Leitura (Read / Pesquisa)** — Para pesquisar e achar os dados do solicitante a partir do telefone do WhatsApp.
- **Chamados (Tickets):** Permissão de **Criação (Create)** e **Atualização (Update)** — Para abrir chamados e registrar as interações subsequentes.
- **Documentos (Documents):** Permissão de **Criação (Create)** — Para subir prints, fotos e PDFs enviados pelo usuário e anexá-los ao ticket.

---

## 🚀 Passo a Passo de Instalação e Funcionamento

A aplicação roda inteiramente via **Docker**, sem necessidade de instalar Go ou qualquer outra dependência.

#### Passo 1: Instalar Docker e Docker Compose
Certifique-se de ter o Docker instalado em sua máquina ou servidor Linux/Windows.

#### Passo 2: Configurar o `docker-compose.yml`
Crie uma pasta no servidor chamada `glpi-bot` e, dentro dela, crie um arquivo chamado `docker-compose.yml` com o seguinte conteúdo:

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
      - ./config:/app/config
```

#### Passo 3: Iniciar o Serviço
Abra o terminal na pasta onde colocou o arquivo e execute:
```bash
docker compose up -d
```
O container iniciará na porta `33090`. Para verificar se está rodando:
```bash
docker compose ps
```

---

## ⚙️ Configuração Inicial e Primeiro Acesso

Uma vez que o bot esteja rodando, siga os passos abaixo para fazê-lo funcionar:

### Passo 1: Entrar no Painel Web
1. Abra o navegador e digite o endereço: `http://localhost:33090` (ou o IP do seu servidor).
2. Insira as credenciais padrão de administração:
   - **Usuário:** `admin`
   - **Senha:** `admin123`
3. _Dica de Segurança:_ Após o primeiro login, acesse a aba **Configurações > Bloqueios & Senha** para mudar a senha de administrador.

### Passo 2: Conectar o WhatsApp
1. Na tela principal (**Status**), se for o primeiro acesso, o painel exibirá o botão **Conectar** e gerará um **QR Code**.
2. Abra o WhatsApp no seu smartphone de atendimento, vá em **Aparelhos Conectados > Conectar um aparelho**.
3. Aponte a câmera do celular para o QR Code gerado na tela do painel.
4. Após o escaneamento bem-sucedido, a tela do painel mudará para o status verde de **Sessão Ativa / Conectado**.

### Passo 3: Configurar os Parâmetros da TI (Aba Configurações)
1. **👥 Identidade & Suporte:** Defina o nome da sua empresa e insira o WhatsApp da TI (com DDI 55 + DDD + Número) para receber notificações. Defina os nomes dos técnicos da fila de transbordo.
2. **🔌 Conexão GLPI API:** Insira o link absoluto do seu GLPI (ex: `https://meu-glpi/apirest.php`), o **App-Token** e o **User-Token** obtidos no GLPI.
3. **📧 SMTP (Opcional):** Se desejar receber alertas por e-mail quando a sessão do WhatsApp desconectar, ative o SMTP e preencha as credenciais. Clique no botão de teste para garantir o funcionamento.
4. **📅 Horário & Ausência:** Defina a faixa horária de suporte (ex: das `08:00` às `18:00`), marque os dias de atendimento e digite a mensagem de ausência automática.
5. **Salvar:** Clique no botão verde superior **Salvar Configurações**.

### Passo 4: Personalizar Mensagens do Bot (Aba Mensagens)
- Acesse a aba **Mensagens** e personalize os textos que o bot enviará nas diferentes fases do atendimento.
- Utilize as variáveis como `{saudacao}`, `{empresa}`, `{ticket_id}` e `{ticket_title}` para criar respostas personalizadas e dinâmicas.
- Clique em **Salvar Mensagens** ao terminar.

### Passo 5: Criar sua Árvore de Atendimento (Aba Fluxo do Bot)
- Acesse a aba **Fluxo do Bot**.
- O menu principal já vem criado. Você pode editar os títulos e escolher o tipo da ação de cada botão.
- Use **Adicionar Opção** para criar novos nós.
- Se for uma opção de tipo **Abrir Chamado GLPI**, lembre-se de configurar o **ID da Categoria** correspondente ao seu catálogo de serviços no GLPI, bem como as mídias requeridas.
- Clique no botão **Salvar Árvore de Fluxo** para atualizar o robô imediatamente.

---

## 🛠️ Resolução de Problemas Comuns

### 1. O QR Code não carrega ou mostra "Erro de Conexão com a API"
- Verifique se a aplicação está rodando. Se rodando via Docker, olhe os logs do container (`docker logs glpi-bot`).
- Caso necessário, acesse **Configurações > Manutenção do Sistema** e clique em **Reiniciar Bot** para recarregar a engine do WhatsApp.

### 2. Os chamados não são criados no GLPI
- Verifique se a URL da API está correta e termina em `/apirest.php`.
- Teste se o servidor onde o bot está rodando consegue alcançar o servidor do GLPI (problemas de rede, regras de firewall ou SSL inválido podem bloquear a conexão).
- Verifique se o App-Token e o User-Token inseridos na aba de configurações estão corretos.
- Verifique se o número de WhatsApp do solicitante está cadastrado no campo "Telefone" do usuário no GLPI (ou se o perfil do usuário da API tem permissão para pesquisar usuários).

### 3. Os arquivos de anexo (Fotos ou Documentos) não são vinculados ao chamado
- Certifique-se de que a opção de chamado no **Fluxo do Bot** está com as caixas "Solicitar Imagens" e/ou "Solicitar Documentos" devidamente marcadas.
- Certifique-se de que o perfil (Profile) do usuário do bot no GLPI tem permissão de escrita/criação na aba **Documentos**.
