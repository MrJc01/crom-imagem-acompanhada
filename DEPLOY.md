# Cron-Vision v4 - Master Deployment Guide

Este guia destina-se ao provisionamento em ambientes de produção (VPS, AWS EC2, DigitalOcean Droplets).

## 1. Requisitos do Servidor

* Docker + Docker Compose instalados.
* NGINX instalado no host (reverso proxy).
* Acesso root ou sudo.

## 2. Estrutura do Projeto (Host)

Clone ou copie a pasta do projeto para o diretório `/opt/crom-vision`:

```bash
mkdir -p /opt/crom-vision
cd /opt/crom-vision
# Baixe ou realize git pull do seu código
```

## 3. Configuração do Ambiente (.env)

Crie um arquivo `.env` definitivo:

```bash
nano /opt/crom-vision/.env
```

Preencha as variáveis de ambiente baseadas no seu `.env.example`:

```env
APP_MODE=saas
APP_SALT=SUA_CHAVE_ALFANUMERICA_MUITO_SECRETA_AQUI
PORT=8080
BASE_URL=https://up.crom.run
STORAGE_PATH=/app/storage
DEFAULT_EXPIRATION_DAYS=7
DATABASE_URL=/app/crom_vision.db

# SMTP
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=marketing@crom.run
SMTP_PASS=**********

# Pagamentos MP (Se aplicável)
MP_ACCESS_TOKEN=APP_USR-*******
```

## 4. O Docker Compose

Assegure que o seu `docker-compose.yml` faz o bind correto dos volumes, principalmente a persistência do banco `crom_vision.db` e das imagens.

```yaml
version: '3.8'

services:
  crom-vision:
    build: .
    container_name: crom_vision_app
    ports:
      - "8080:8080"
    volumes:
      - ./.env:/app/.env
      - ./data:/app/storage    # Persiste as imagens físicas enviadas
    restart: unless-stopped
    environment:
      - TZ=America/Sao_Paulo
```

Importante: O SQLite cria os arquivos `-wal` e `-shm`. Por isso, evite montar o DB diretamente como arquivo isolado `crom_vision.db:/app/crom_vision.db`. É preferível apontê-lo pra dentro de uma pasta (ex: `/app/database`) no script main, ou deixá-lo no escopo geral como mapeamento da home raiz.

## 5. Subindo a Aplicação

```bash
cd /opt/crom-vision
docker-compose up -d --build
```
Aguarde. Você pode auditar os logs com `docker-compose logs -f crom-vision`.

## 6. Configurando NGINX (Proxy Reverso e SSL)

```bash
sudo nano /etc/nginx/sites-available/crom-vision
```

Adicione:

```nginx
server {
    server_name up.crom.run; # Troque pelo seu domínio

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
                 
        # Aumentar tamanho máximo de upload (Snipboard)
        client_max_body_size 15M;
    }
}
```

Habilite e reinicie:

```bash
sudo ln -s /etc/nginx/sites-available/crom-vision /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

## 7. Gerando o Certificado SSL

```bash
sudo certbot --nginx -d up.crom.run
```
