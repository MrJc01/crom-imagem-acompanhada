#!/usr/bin/env bash
# ============================================================
# Imagem Acompanhada — Teste de Upload de Imagem
# Testa o fluxo completo: upload → preview → pixel
# ============================================================
set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}🖼️  Imagem Acompanhada — Teste de Upload de Imagem${NC}"
echo "================================================"

# Criar imagem PNG mínima de teste (1x1 pixel vermelho)
TMP_IMG=$(mktemp /tmp/crom_test_XXXX.png)
# PNG 1x1 pixel em binário
printf '\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x02\x00\x00\x00\x90wS\xde\x00\x00\x00\x0cIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82' > "$TMP_IMG"

echo -e "\n📤 Fazendo upload da imagem de teste..."
RESP=$(curl -s -X POST "$BASE_URL/api/checkout" \
    -F "tier=3d" \
    -F "email=upload-test@crom.run" \
    -F "max_views=50" \
    -F "image=@$TMP_IMG")

echo "Resposta: $RESP"

ID=$(echo "$RESP" | jq -r '.id // empty')
PIXEL=$(echo "$RESP" | jq -r '.pixel_url // empty')
PREVIEW=$(echo "$RESP" | jq -r '.preview_url // empty')
STATUS=$(echo "$RESP" | jq -r '.payment_status // empty')

echo ""
if [ -n "$ID" ]; then
    echo -e "${GREEN}✅ Upload OK${NC} — ID: $ID"
else
    echo -e "${RED}❌ Upload falhou${NC}"
    rm -f "$TMP_IMG"
    exit 1
fi

echo -e "   Pixel URL:   $PIXEL"
echo -e "   Preview URL: $PREVIEW"
echo -e "   Status:      $STATUS"

# Testar preview
echo -e "\n🔍 Testando preview..."
PREV_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/p/$ID")
if [ "$PREV_CODE" = "200" ]; then
    echo -e "${GREEN}✅ Preview acessível (200)${NC}"
else
    echo -e "${RED}❌ Preview falhou (HTTP $PREV_CODE)${NC}"
fi

# Testar pixel tracker
echo -e "\n📡 Testando pixel tracker..."
PIX_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/i/$ID")
if [ "$PIX_CODE" = "200" ]; then
    echo -e "${GREEN}✅ Pixel tracker respondeu (200)${NC}"
else
    echo -e "${RED}❌ Pixel tracker falhou (HTTP $PIX_CODE)${NC}"
fi

# Verificar contagem
echo -e "\n📊 Verificando stats..."
sleep 1
STATS=$(curl -s "$BASE_URL/api/link-stats?id=$ID&period=24h")
echo "Stats: $STATS"

# Limpeza
rm -f "$TMP_IMG"
echo -e "\n${GREEN}✅ Teste de upload concluído${NC}"
