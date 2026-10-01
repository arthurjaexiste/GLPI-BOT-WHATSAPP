# GLPI-BOT

Bot de atendimento via WhatsApp integrado à API do GLPI, com suporte ao vivo (live chat), abertura e consulta de chamados por enquetes interativas e painel web de gerenciamento.

## Como rodar com Docker

Não é necessário clonar o código-fonte nem instalar Go. Basta ter o Docker instalado e rodar a imagem oficial.

### Opção 1: Docker Compose (recomendado)

Crie um diretório para o bot, salve o arquivo `docker-compose.yml` abaixo e inicie o serviço:

```yaml
services:
  glpi-bot:
    image: ghcr.io/arthurjaexiste/glpi-bot-whatsapp:latest
    container_name: glpi-bot
    restart: unless-stopped
    ports:
      - "33090:33090"
    environment:
      - TZ=America/Sao_Paulo
    volumes:
      - ./db:/app/db
```

Inicie o container:

```bash
docker compose up -d
```

### Opção 2: Docker Run

Se preferir rodar direto via linha de comando:

```bash
docker run -d \
  --name glpi-bot \
  --restart unless-stopped \
  -p 33090:33090 \
  -e TZ=America/Sao_Paulo \
  -v $(pwd)/db:/app/db \
  ghcr.io/arthurjaexiste/glpi-bot-whatsapp:latest
```

## Primeiro Acesso e Configuração

Após subir o container, acesse o painel web:

**URL:** `http://localhost:33090` (ou pelo IP do servidor)

1. **Conta de Administrador**: No primeiro acesso, defina o usuário e senha do administrador inicial na tela de boas-vindas.
2. **Conectar WhatsApp**: Na aba **Status**, clique no botão **Conectar** e escaneie o QR Code com o WhatsApp da sua operação.
3. **Integrar ao GLPI**: Na aba **Configurações**, informe a URL da API REST do GLPI (`https://seu-glpi/apirest.php`), o **App-Token** e o **User-Token**.
4. **Customizar Mensagens e Fluxos**: Ajuste mensagens automáticas, horário de expediente e as opções das enquetes interativas diretamente pelo painel.

> Todos os dados (sessão do WhatsApp, usuários e configurações) são salvos no diretório `./db` montado pelo Docker, persistindo mesmo após recriar o container.

## Principais Recursos

- **Abertura e consulta de chamados**: Integração direta com a API REST do GLPI.
- **Enquetes interativas**: Navegação limpa no WhatsApp usando as enquetes nativas.
- **Live Chat com RBAC**: Painel de atendimento em tempo real com perfis de Administrador e Operador.
- **Configuração 100% via Web**: Sem arquivos `.env` manuais — tudo é gerenciado e salvo pelo painel.

