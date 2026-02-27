package handlers

import (
	"encoding/json"
	"net/http"

	"crom-vision/internal/database"
)

// PaymentInfoHandler retorna dados de pagamento do servidor (sem expor preço na URL)
// GET /api/payment-info?id=xxx
func PaymentInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	linkID := r.URL.Query().Get("id")
	if linkID == "" {
		http.Error(w, "ID obrigatório", http.StatusBadRequest)
		return
	}

	var (
		price          float64
		paymentStatus  string
		mpQRBase64     *string
		mpQRCode       *string
		mpTicketURL    *string
	)

	err := database.DB.QueryRow(`
		SELECT price, payment_status, mp_qr_base64, mp_qr_code, mp_ticket_url
		FROM links WHERE id = ?`, linkID).Scan(
		&price, &paymentStatus, &mpQRBase64, &mpQRCode, &mpTicketURL,
	)

	if err != nil {
		http.Error(w, "Link não encontrado", http.StatusNotFound)
		return
	}

	result := map[string]interface{}{
		"id":     linkID,
		"price":  price,
		"status": paymentStatus,
	}

	if mpQRBase64 != nil {
		result["qr_code_base64"] = *mpQRBase64
	}
	if mpQRCode != nil {
		result["qr_code"] = *mpQRCode
	}
	if mpTicketURL != nil {
		result["ticket_url"] = *mpTicketURL
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
