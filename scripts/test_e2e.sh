#!/usr/bin/env bash
# ============================================================
# Imagem Acompanhada — Script de Teste End-to-End
# Requer: curl, jq, servidor rodando em localhost:8080
# ============================================================
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
PASS=0
FAIL=0
TOTAL=0

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo -e "${GREEN}✅ PASS${NC}: $1"; }
log_fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo -e "${RED}❌ FAIL${NC}: $1 — $2"; }

echo -e "${YELLOW}🧪 Imagem Acompanhada E2E Tests — $BASE_URL${NC}"
echo "================================================"

# --- 1. Health Check: Servidor Online ---
echo ""
echo "📋 Seção 1: Conectividade"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/")
if [ "$HTTP_CODE" = "200" ]; then
    log_pass "Landing page acessível (200)"
else
    log_fail "Landing page" "HTTP $HTTP_CODE"
    echo -e "${RED}ABORTANDO: Servidor não está respondendo.${NC}"
    exit 1
fi

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/config")
if [ "$HTTP_CODE" = "200" ]; then
    log_pass "/api/config acessível (200)"
else
    log_fail "/api/config" "HTTP $HTTP_CODE"
fi

# --- 2. Checkout Modo Free (sem imagem) ---
echo ""
echo "📋 Seção 2: Checkout Modo Free"
CHECKOUT_RESP=$(curl -s -X POST "$BASE_URL/api/checkout" \
    -F "tier=1d" \
    -F "email=e2e-test@crom.run" \
    -F "max_views=50")

LINK_ID=$(echo "$CHECKOUT_RESP" | jq -r '.id // empty')
PIXEL_URL=$(echo "$CHECKOUT_RESP" | jq -r '.pixel_url // empty')
PAY_STATUS=$(echo "$CHECKOUT_RESP" | jq -r '.payment_status // empty')

if [ -n "$LINK_ID" ]; then
    log_pass "Checkout retornou ID: $LINK_ID"
else
    log_fail "Checkout" "ID não retornado — resp: $CHECKOUT_RESP"
fi

if [ "$PAY_STATUS" = "approved" ]; then
    log_pass "Payment status = approved (modo free)"
else
    log_fail "Payment status" "esperava approved, obteve: $PAY_STATUS"
fi

if [ -n "$PIXEL_URL" ]; then
    log_pass "Pixel URL gerada: $PIXEL_URL"
else
    log_fail "Pixel URL" "não retornada"
fi

# --- 3. Tracker Pixel ---
echo ""
echo "📋 Seção 3: Tracker Pixel"
if [ -n "$LINK_ID" ]; then
    # Acessar o pixel
    PIXEL_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/i/$LINK_ID")
    if [ "$PIXEL_CODE" = "200" ]; then
        log_pass "Pixel GET /i/$LINK_ID retornou 200"
    else
        log_fail "Pixel GET" "HTTP $PIXEL_CODE"
    fi

    # Pixel com ID inexistente
    PIXEL_404=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/i/naoexiste999")
    if [ "$PIXEL_404" = "200" ]; then
        log_pass "Pixel inexistente retorna 200 (GIF transparente)"
    else
        log_fail "Pixel inexistente" "HTTP $PIXEL_404"
    fi
fi

# --- 4. Public Links ---
echo ""
echo "📋 Seção 4: Links Públicos"
PUB_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/public-links")
if [ "$PUB_CODE" = "200" ]; then
    log_pass "/api/public-links retornou 200"
else
    log_fail "/api/public-links" "HTTP $PUB_CODE"
fi

PUB_BODY=$(curl -s "$BASE_URL/api/public-links")
if echo "$PUB_BODY" | jq '.' > /dev/null 2>&1; then
    log_pass "/api/public-links retornou JSON válido"
else
    log_fail "/api/public-links JSON" "resposta inválida"
fi

# --- 5. Link Stats ---
echo ""
echo "📋 Seção 5: Stats do Link"
if [ -n "$LINK_ID" ]; then
    for PERIOD in 10m 1h 24h 7d; do
        STAT_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/link-stats?id=$LINK_ID&period=$PERIOD")
        if [ "$STAT_CODE" = "200" ]; then
            log_pass "link-stats period=$PERIOD retornou 200"
        else
            log_fail "link-stats period=$PERIOD" "HTTP $STAT_CODE"
        fi
    done

    # Período inválido
    STAT_INV=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/link-stats?id=$LINK_ID&period=99h")
    if [ "$STAT_INV" = "400" ]; then
        log_pass "link-stats period=99h retornou 400 (esperado)"
    else
        log_fail "link-stats período inválido" "HTTP $STAT_INV (esperava 400)"
    fi
fi

# --- 6. Geo Stats ---
echo ""
echo "📋 Seção 6: Geo Stats"
if [ -n "$LINK_ID" ]; then
    GEO_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/link-geo?id=$LINK_ID")
    if [ "$GEO_CODE" = "200" ]; then
        log_pass "/api/link-geo retornou 200"
    else
        log_fail "/api/link-geo" "HTTP $GEO_CODE"
    fi
fi

# --- 7. Webhook de Pagamento ---
echo ""
echo "📋 Seção 7: Webhook MercadoPago"
WH_CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/api/webhook/mp" \
    -H "Content-Type: application/json" \
    -d "{\"link_id\": \"fake_test_id\", \"status\": \"approved\"}")
if [ "$WH_CODE" = "200" ]; then
    log_pass "Webhook POST retornou 200"
else
    log_fail "Webhook POST" "HTTP $WH_CODE"
fi

# --- 8. Checkout inválido ---
echo ""
echo "📋 Seção 8: Validações"
INV_METHOD=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/checkout")
if [ "$INV_METHOD" = "405" ]; then
    log_pass "GET /api/checkout retornou 405 (Method Not Allowed)"
else
    log_fail "GET /api/checkout" "HTTP $INV_METHOD (esperava 405)"
fi

# --- Resultados Finais ---
echo ""
echo "================================================"
echo -e "${YELLOW}📊 RESULTADO FINAL${NC}"
echo -e "  Total:  $TOTAL"
echo -e "  ${GREEN}Pass:   $PASS${NC}"
echo -e "  ${RED}Fail:   $FAIL${NC}"
echo "================================================"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
