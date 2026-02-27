package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"crom-vision/internal/database"
	"crom-vision/internal/services"
)

// WebhookMPHandler recebe notificações do Mercado Pago (IPN)
// Suporta tanto o formato simplificado (dev mode) quanto IPN real
func WebhookMPHandler(w http.ResponseWriter, r *http.Request) {
	// Formato simplificado (dev mode): {"link_id": "xxx", "status": "approved"}
	var notification struct {
		LinkID string `json:"link_id"`
		Status string `json:"status"` // "approved"
	}

	// Tenta parsear como IPN real do Mercado Pago
	// MP envia: {"action": "payment.updated", "data": {"id": "123456"}}
	var mpNotification struct {
		Action string `json:"action"`
		Data   struct {
			ID string `json:"id"`
		} `json:"data"`
		Type string `json:"type"`
	}

	bodyBytes := make([]byte, r.ContentLength)
	r.Body.Read(bodyBytes)

	// Primeiro tenta IPN real
	if err := json.Unmarshal(bodyBytes, &mpNotification); err == nil && mpNotification.Data.ID != "" {
		log.Printf("[MP IPN] Recebido: action=%s, payment_id=%s", mpNotification.Action, mpNotification.Data.ID)
		
		// Consultar status real do pagamento no MP
		paymentID, _ := strconv.ParseInt(mpNotification.Data.ID, 10, 64)
		if paymentID > 0 {
			status, err := services.GetPaymentStatus(paymentID)
			if err != nil {
				log.Printf("[MP ERR] Falha ao consultar pagamento %d: %v", paymentID, err)
				w.WriteHeader(http.StatusOK) // Responde OK pro MP não reenviar
				return
			}

			if status == "approved" {
				// Buscar link pelo mp_payment_id
				mpIDStr := fmt.Sprintf("%d", paymentID)
				var linkID string
				err := database.DB.QueryRow("SELECT id FROM links WHERE mp_payment_id = ?", mpIDStr).Scan(&linkID)
				if err != nil {
					log.Printf("[MP WARN] Pagamento %s aprovado mas link não encontrado no DB", mpIDStr)
					w.WriteHeader(http.StatusOK)
					return
				}
				
				activateLink(linkID)
			}
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	// Fallback: formato dev mode simplificado
	if err := json.Unmarshal(bodyBytes, &notification); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if notification.Status == "approved" && notification.LinkID != "" {
		activateLink(notification.LinkID)
		log.Printf("[💰 DEV] Link %s liberado via webhook simplificado!", notification.LinkID)
	}
	w.WriteHeader(http.StatusOK)
}

// activateLink marca um link como aprovado e notifica o dono
func activateLink(linkID string) {
	database.DB.Exec("UPDATE links SET payment_status = 'approved' WHERE id = ?", linkID)

	var email sql.NullString
	err := database.DB.QueryRow("SELECT email FROM links WHERE id = ?", linkID).Scan(&email)
	if err == nil && email.Valid && email.String != "" {
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
		go services.SendEmail(email.String, "Crom-Vision - Pagamento Aprovado!", fmt.Sprintf(`<h2>Tudo Certo!</h2>
		<p>Seu proxy tracker <b>%s</b> foi pago e ativado com sucesso.</p>
		<p>Acesse seu Dashboard: <a href="%s/?dashboard=%s">%s/?dashboard=%s</a></p>`, linkID, baseURL, linkID, baseURL, linkID))
	}
	log.Printf("[💰 Pagamento Ativado] Link %s liberado!", linkID)
}
