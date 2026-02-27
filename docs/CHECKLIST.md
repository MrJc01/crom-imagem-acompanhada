# 📋 Checklist de Implementação: Crom-Vision (V3: Híbrido & SMTP)

## 🛠️ Infraestrutura & Config
- [x] Criar variável `APP_MODE` no `.env` (alternar entre 'saas' e 'free').
- [x] Adicionar `BASE_URL` para gerar links absolutos (ex: `https://seu-pixel.com/i/123`).
- [x] Configurar credenciais SMTP do Gmail (Host: smtp.gmail.com, Port: 587).
- [x] Criar volume no `docker-compose.yml` para persistir o `crom_vision.db`.

## 📧 Comunicação (Gmail)
- [x] Criar função de envio de e-mail em Go (`net/smtp`).
- [x] Template de e-mail: Boas-vindas + ID do Ativo + Senha do Painel.
- [x] Lógica para disparar e-mail assim que o pagamento for aprovado (ou no checkout free).

## 💰 Lógica de Negócio (Checkout)
- [x] Se `APP_MODE == free`: pular integração de pagamento e marcar como approved.
- [x] Se `APP_MODE == saas`: manter fluxo de pending até o webhook do MercadoPago.
- [x] Implementar seletor de dias customizado para o modo gratuito no frontend.

## 🎨 Interface (Tailwind CSS)
- [x] Refatorar landing page (`index.html`) para mostrar "Criar meu Pixel" em destaque.
- [x] Adicionar feedback visual (Toasts ou Modais) ao concluir o checkout.
- [x] Painel Privado (`private.html`): Adicionar botão "Copiar Link do Pixel" e "Copiar Link do Redirect".

## 📄 Documentação & Manuais
- [x] Gerar `DEPLOY.md`: Passo a passo do docker build e docker run.
- [x] Atualizar `README.md`: Lista de todos os novos parâmetros do `.env`.
- [x] Atualizar `ARCHITECTURE.md`: Explicar o novo fluxo de e-mail e modos de operação.
