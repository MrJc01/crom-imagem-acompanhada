package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"crom-vision/internal/database"
	"crom-vision/internal/services"
)

func WebhookMPHandler(w http.ResponseWriter, r *http.Request) {
	var notification struct {
		LinkID string `json:"link_id"`
		Status string `json:"status"` // "approved"
	}
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if notification.Status == "approved" {
		database.DB.Exec("UPDATE links SET payment_status = 'approved' WHERE id = ?", notification.LinkID)
		
		var email sql.NullString
		// Notifica o cliente final via webhook q acabou de ser aprovado e liberou!
		err := database.DB.QueryRow("SELECT email FROM links WHERE id = ?", notification.LinkID).Scan(&email)
		if err == nil && email.Valid && email.String != "" {
			baseURL := os.Getenv("BASE_URL")
			if baseURL == "" { baseURL = "http://localhost:8080" }
			go services.SendEmail(email.String, "Crom-Vision - Pagamento Aprovado!", fmt.Sprintf(`<h2>Tudo Certo!</h2><p>Seu proxy tracker <b>%s</b> foi pago e ativado com sucesso. Você já pode integrá-lo no TabNews ou Github.</p>`, notification.LinkID))
		}
		log.Printf("[💰 Pagamento Ativado] Link %s liberado via webhook!", notification.LinkID)
	}
	w.WriteHeader(http.StatusOK)
}
