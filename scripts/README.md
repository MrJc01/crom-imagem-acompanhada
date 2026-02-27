# 🧪 Scripts de Teste — Imagem Acompanhada

## Visão Geral

| Script | Tipo | Requer Servidor? | Descrição |
|---|---|---|---|
| `run_all_tests.sh` | Master | Opcional | Roda unit tests + E2E (se servidor ativo) |
| `test_e2e.sh` | E2E | ✅ Sim | 18 checks em todos os endpoints da API |
| `test_upload.sh` | E2E | ✅ Sim | Fluxo completo: upload → preview → pixel |

---

## 🚀 Início Rápido

### Testes Unitários (não precisa de servidor)

```bash
# Rodar todos os testes Go
go test -v ./...

# Com detecção de race condition
go test -v -race ./...

# Pacote específico
go test -v ./internal/utils/
go test -v ./internal/handlers/
go test -v ./internal/services/
go test -v ./internal/database/
```

### Testes E2E (precisa do servidor rodando)

Os scripts E2E fazem requisições HTTP reais. Por isso, o servidor precisa estar rodando **em outro terminal** antes de executá-los.

```bash
# Terminal 1 — Iniciar o servidor
go run ./cmd/server

# Terminal 2 — Rodar os testes E2E
bash scripts/test_e2e.sh
bash scripts/test_upload.sh
```

### Script Master (faz tudo)

```bash
bash scripts/run_all_tests.sh
```

> Este script roda `go vet` + `go test` automaticamente. Se detectar o servidor em `localhost:8080`, também roda os testes E2E. Se não, pula a parte E2E e avisa.

---

## 📋 Detalhes dos Scripts

### `run_all_tests.sh`

**Fase 1** — Testes Unitários:
- `go vet ./...` (análise estática)
- `go test -v -count=1 -race ./...` (todos os testes com race detector)

**Fase 2** — Testes E2E (só se servidor estiver rodando):
- Executa `test_e2e.sh` e `test_upload.sh`

---

### `test_e2e.sh`

Testa **todos os endpoints da API** com `curl`. Aceita URL base como argumento:

```bash
# Padrão: localhost:8080
bash scripts/test_e2e.sh

# Servidor custom
bash scripts/test_e2e.sh https://up.crom.run
```

**O que testa:**
1. Landing page acessível (`GET /`)
2. Config API (`GET /api/config`)
3. Checkout modo Free (`POST /api/checkout`)
4. Tracker pixel (`GET /i/:id`)
5. Pixel inexistente (deve ser GIF transparente)
6. Links públicos (`GET /api/public-links`)
7. Stats em todos os períodos: `10m`, `1h`, `24h`, `7d`
8. Período inválido (deve retornar 400)
9. Geo stats (`GET /api/link-geo`)
10. Webhook MercadoPago (`POST /api/webhook/mp`)
11. Validação de método (`GET /api/checkout` → 405)

**Saída:** Relatório colorido com contagem PASS/FAIL.

---

### `test_upload.sh`

Testa o **fluxo completo de upload de imagem**:

```bash
bash scripts/test_upload.sh
# ou com URL custom:
bash scripts/test_upload.sh https://up.crom.run
```

**O que faz:**
1. Cria um PNG mínimo (1x1 pixel) temporário
2. Faz upload via `POST /api/checkout` com `tier=3d`
3. Testa preview da imagem (`GET /p/:id`)
4. Testa pixel tracker (`GET /i/:id`)
5. Verifica stats do link
6. Limpa arquivo temporário

---

## 📊 Cobertura dos Testes Unitários

| Pacote | Arquivo | Testes | O que cobre |
|---|---|---|---|
| `utils` | `crypto_test.go` | 7 | Random string, fingerprint hash, salt |
| `utils` | `ip_test.go` | 6 | GeoIP simulado, Anti-F5, cooldown, concorrência |
| `utils` | `image_test.go` | 4 | GIF89a válido, 1x1 pixel |
| `database` | `db_test.go` | 7 | Schema, colunas, CRUD, migrações |
| `handlers` | `handlers_test.go` | 14 | Checkout, image proxy, preview |
| `handlers` | `stats_test.go` | 12 | Public links, stats, geo, private stats |
| `handlers` | `webhooks_test.go` | 5 | Aprovação, rejeição, JSON inválido |
| `services` | `background_test.go` | 3 | Hard delete sweeper, expiração |
| `services` | `email_test.go` | 3 | Skip SMTP, fallback, invalid config |
| | | **61** | **Total** |

---

## 🔧 Pré-requisitos

- **Go 1.22+** instalado
- **curl** e **jq** (para scripts E2E)
- GCC/musl-dev (para compilar `go-sqlite3` com CGO)

```bash
# Instalar jq (se necessário)
sudo apt install jq    # Debian/Ubuntu
brew install jq        # macOS
```

---

## ❓ Problemas Comuns

### "exit code 7" ao rodar tudo junto
Os scripts E2E precisam que o servidor esteja rodando. Se você rodar tudo de uma vez sem servidor, os scripts E2E vão falhar. **Solução**: rode os comandos separadamente:

```bash
# 1. Unit tests (sempre funciona)
go test -v ./...

# 2. E2E (precisa do servidor em outro terminal)
go run ./cmd/server &
sleep 2
bash scripts/test_e2e.sh
```

### "cached" nos resultados
Normal. O Go cache os resultados de testes que não mudaram. Para forçar re-execução:

```bash
go test -count=1 -v ./...
```
