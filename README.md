# Imagem Acompanhada 👁️

**Analytics de Imagens e Proxies temporários privacy-first.**

Um micro-serviço em Go criado para embutir rastreamentos de campanhas, fóruns (como TabNews, GitHub) ou e-mails, transformando uma simples imagem num motor de Analytics Completo.

O Imagem Acompanhada v4 Híbrido atua em dois modos:
1. **Modo Snipboard (Free)**: O usuário apenas solta a imagem no "Dropzone" (`Ctrl+V`) e recebe tanto uma URL pública (`/p/:id`) quanto um Tracker Proxy (`/i/:id`), expirando de acordo com os dias que ele configurou em tela. Gravando a imagem real no disco (diretório `./storage`).
2. **Modo Pixel SaaS**: Permite integração via Webhook do MercadoPago para gerar cobrança baseada no "Tempo Restante" e "Máximo de Visualizações". Entra com status Pendente e exibe uma Imagem Branca no lugar.

O sistema é blindado usando rotinas de vassoura (`os.Remove(filePath)`) sempre que uma validade é vencida e salva fingerprints criptográficos (`Hash+SALT`) protegendo a LGPD no lugar do IP nominal.

## Como Iniciar

1. Clone e configure as chaves essenciais:
   ```bash
   cp .env.example .env
   ```
   **Principais Variáveis:**
   - `APP_MODE`: `saas` ou `free`.
   - `BASE_URL`: (ex: `https://crom.run`) base das URLs geradas.
   - `STORAGE_PATH`: (Opcional, default: `./storage`) Caminho para os arquivos físicos das imagens salvas.
   - `SMTP_USER` / `SMTP_PASS`: E-mail remetente e _App Password_.

2. Suba o container (Ideal) ou o Server Go (Local):
   ```bash
   # Opção Docker:
   docker-compose up -d --build
   
   # Opção Local Dev:
   go mod tidy
   go run main.go
   ```

A dashboard estará acessível em `http://localhost:8080/`.
