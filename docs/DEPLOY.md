# 🚀 Guia de Deploy (Crom-Vision v3)

Este documento descreve como realizar o setup profissional do Crom-Vision em um ambiente VPS (Ubuntu/Mint) utilizando Docker, garantindo persistência do SQLite e segurança via SSL (Nginx).

## 1. Preparando o Ambiente
Garanta que seu servidor possui o Docker e o Nginx instalados.
```bash
sudo apt update
sudo apt install docker.io docker-compose nginx certbot python3-certbot-nginx
```

## 2. Configurando o Repositório e `.env`
Clone o repositório na sua VPS:
```bash
git clone https://github.com/seu-user/crom-imagem-acompanhada.git crom-vision
cd crom-vision
cp .env.example .env
```
**Importante:** Ajuste o `.env` definindo o `APP_MODE=saas` (ou `free`), adicione `STORAGE_PATH=/app/data/uploads` para fixar as gravações de imagem no volume, a `BASE_URL` (ex: `https://crom.run`) e suas credenciais de `SMTP` do Gmail. (Utilize uma **App Password** do Google, não sua senha real).

## 3. Subindo o Container (Docker)
O `docker-compose.yml` mapeia um volume local `./data` para `/app/data` no container, protegendo o seu banco SQLite de apagamentos acidentais durante deploys.
```bash
# Cria a pasta de dados vazia para evitar problemas de permissão root do docker
mkdir data
chmod 777 data

# Constrói e sobe em background
docker-compose up -d --build
```
Verifique os logs para garantir que a `database v2` migrou corretamente:
```bash
docker-compose logs -f
```

## 4. Reverse Proxy e SSL (Nginx)
Queremos que `https://crom.run/` aponte para o nosso container interno na porta `8080`.

Crie um arquivo de site no Nginx:
```bash
sudo nano /etc/nginx/sites-available/crom-vision
```
E insira o bloco:
```nginx
server {
    listen 80;
    server_name crom.run; # Troque pelo seu domínio/subdomínio

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        client_max_body_size 10M; # Fundamental para permitir o upload de imagens
    }
}
```

Ative o site e reinicie o Nginx:
```bash
sudo ln -s /etc/nginx/sites-available/crom-vision /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

## 5. Gerando o SSL (HTTPS Gratuito)
Com o tráfego rodando, peça ao Certbot para emitir seu certificado e reescrever o Nginx automaticamente para forçar HTTPS:
```bash
sudo certbot --nginx -d crom.run
```

Feito! O Crom-Vision SaaS agora roda seguro com a base de dados blindada e e-mails disparando localmente.
