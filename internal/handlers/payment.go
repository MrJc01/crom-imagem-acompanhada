package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"crom-vision/internal/database"
	"crom-vision/internal/services"
)

// PaymentInfoHandler retorna dados de pagamento do servidor (sem expor preço na URL)
// GET /api/payment-info?id=xxx
// Quando o status no DB é "pending" e há mp_payment_id, consulta o Mercado Pago
// para verificar se o pagamento foi aprovado (polling ativo server-side)
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
		price         float64
		paymentStatus string
		mpPaymentID   *string
		mpQRBase64    *string
		mpQRCode      *string
		mpTicketURL   *string
	)

	err := database.DB.QueryRow(`
		SELECT price, payment_status, mp_payment_id, mp_qr_base64, mp_qr_code, mp_ticket_url
		FROM links WHERE id = ?`, linkID).Scan(
		&price, &paymentStatus, &mpPaymentID, &mpQRBase64, &mpQRCode, &mpTicketURL,
	)

	if err != nil {
		http.Error(w, "Link não encontrado", http.StatusNotFound)
		return
	}

	// --- Polling ativo: se pendente e tem mp_payment_id, consulta o MP ---
	if paymentStatus == "pending" && mpPaymentID != nil && *mpPaymentID != "" {
		mpID, parseErr := strconv.ParseInt(*mpPaymentID, 10, 64)
		if parseErr == nil && mpID > 0 {
			realStatus, checkErr := services.GetPaymentStatus(mpID)
			if checkErr == nil && realStatus == "approved" {
				// Atualiza o DB e ativa o link
				database.DB.Exec("UPDATE links SET payment_status = 'approved' WHERE id = ?", linkID)
				paymentStatus = "approved"
				log.Printf("[MP POLL ✓] Pagamento %d aprovado via polling para link %s", mpID, linkID)

				// Dispara notificação (mesma lógica do webhook)
				go activateLink(linkID)
			} else if checkErr == nil && realStatus != "" {
				log.Printf("[MP POLL] Pagamento %d status: %s (link %s)", mpID, realStatus, linkID)
			} else if checkErr != nil {
				log.Printf("[MP POLL ERR] Falha ao consultar pagamento %d: %v", mpID, checkErr)
			}
		}
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

	// Se tem mp_payment_id, retorna para debug
	if mpPaymentID != nil && *mpPaymentID != "" {
		result["mp_payment_id"] = *mpPaymentID
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// CheckPaymentHandler permite verificar manualmente o status de um pagamento
// POST /api/check-payment?id=xxx
func CheckPaymentHandler(w http.ResponseWriter, r *http.Request) {
	linkID := r.URL.Query().Get("id")
	if linkID == "" {
		http.Error(w, "ID obrigatório", http.StatusBadRequest)
		return
	}

	var mpPaymentIDStr *string
	var paymentStatus string

	err := database.DB.QueryRow("SELECT payment_status, mp_payment_id FROM links WHERE id = ?", linkID).Scan(&paymentStatus, &mpPaymentIDStr)
	if err != nil {
		http.Error(w, "Link não encontrado", http.StatusNotFound)
		return
	}

	if paymentStatus == "approved" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "approved", "message": "Já aprovado"})
		return
	}

	if mpPaymentIDStr == nil || *mpPaymentIDStr == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "pending", "message": "Sem payment_id do MP"})
		return
	}

	mpID, _ := strconv.ParseInt(*mpPaymentIDStr, 10, 64)
	realStatus, checkErr := services.GetPaymentStatus(mpID)
	if checkErr != nil {
		http.Error(w, fmt.Sprintf("Erro ao consultar MP: %v", checkErr), http.StatusInternalServerError)
		return
	}

	if realStatus == "approved" {
		database.DB.Exec("UPDATE links SET payment_status = 'approved' WHERE id = ?", linkID)
		go activateLink(linkID)
		log.Printf("[MP CHECK ✓] Pagamento %d aprovado via check manual para link %s", mpID, linkID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": realStatus, "mp_payment_id": *mpPaymentIDStr})
}
