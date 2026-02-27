#!/usr/bin/env bash
# ============================================================
# Imagem Acompanhada — Rodar Todos os Testes
# ============================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

echo -e "${CYAN}╔══════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║   🧪 Imagem Acompanhada — Suite de Testes       ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════╝${NC}"

# ---- Fase 1: Testes Unitários Go ----
echo ""
echo -e "${YELLOW}═══ FASE 1: Testes Unitários (go test) ═══${NC}"
echo ""

cd "$PROJECT_DIR"

echo -e "${CYAN}▶ go vet ./...${NC}"
if go vet ./... 2>&1; then
    echo -e "${GREEN}✅ go vet OK${NC}"
else
    echo -e "${RED}❌ go vet falhou${NC}"
fi

echo ""
echo -e "${CYAN}▶ go test -v -count=1 ./...${NC}"
echo ""
if go test -v -count=1 ./... 2>&1; then
    echo -e "\n${GREEN}✅ Todos os testes unitários passaram!${NC}"
else
    echo -e "\n${RED}❌ Alguns testes falharam!${NC}"
    exit 1
fi

# ---- Fase 2: Testes E2E (Opcional) ----
echo ""
echo -e "${YELLOW}═══ FASE 2: Testes E2E (requer servidor) ═══${NC}"
echo ""

# Checar se servidor está rodando
if curl -s -o /dev/null -w "" http://localhost:8080/ > /dev/null 2>&1; then
    echo -e "${GREEN}Servidor detectado em localhost:8080 — rodando testes E2E...${NC}"
    echo ""
    bash "$SCRIPT_DIR/test_e2e.sh"
    echo ""
    bash "$SCRIPT_DIR/test_upload.sh"
else
    echo -e "${YELLOW}⚠️  Servidor não detectado em localhost:8080${NC}"
    echo -e "   Para rodar testes E2E, primeiro inicie o servidor:"
    echo -e "   ${CYAN}go run ./cmd/server${NC}"
    echo -e "   Depois rode: ${CYAN}bash scripts/test_e2e.sh${NC}"
fi

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║   ✅ Suite de Testes Concluída            ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════╝${NC}"
