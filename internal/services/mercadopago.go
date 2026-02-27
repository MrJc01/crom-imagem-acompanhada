package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"crypto/rand"
	mrand "math/rand"
)

// PixPaymentResult contém os dados retornados pelo Mercado Pago após criar um pagamento PIX
type PixPaymentResult struct {
	PaymentID  int64  `json:"payment_id"`
	QRCodeB64  string `json:"qr_code_base64"`
	QRCode     string `json:"qr_code"`
	TicketURL  string `json:"ticket_url"`
	Status     string `json:"status"`
}

// CreatePixPayment cria um pagamento PIX no Mercado Pago via API REST
func CreatePixPayment(amount float64, description, payerEmail, externalRef string) (*PixPaymentResult, error) {
	accessToken := os.Getenv("MP_ACCESS_TOKEN")
	if accessToken == "" {
		return nil, fmt.Errorf("MP_ACCESS_TOKEN não configurado no .env")
	}

	// Gerar idempotency key única
	idempotencyKey := generateIdempotencyKey()

	payload := map[string]interface{}{
		"transaction_amount": amount,
		"description":       description,
		"payment_method_id": "pix",
		"payer": map[string]interface{}{
			"email": payerEmail,
		},
		"external_reference": externalRef,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("falha ao serializar payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.mercadopago.com/v1/payments", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("falha ao criar request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Idempotency-Key", idempotencyKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha na requisição ao Mercado Pago: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		log.Printf("[MP ERR] Status %d — Body: %s", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("Mercado Pago retornou status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse da resposta do Mercado Pago
	var mpResp struct {
		ID                int64  `json:"id"`
		Status            string `json:"status"`
		PointOfInteraction struct {
			TransactionData struct {
				QRCodeBase64 string `json:"qr_code_base64"`
				QRCode       string `json:"qr_code"`
				TicketURL    string `json:"ticket_url"`
			} `json:"transaction_data"`
		} `json:"point_of_interaction"`
	}

	if err := json.Unmarshal(respBody, &mpResp); err != nil {
		return nil, fmt.Errorf("falha ao parsear resposta do MP: %w", err)
	}

	log.Printf("[MP ✓] Pagamento PIX criado — ID: %d, Status: %s, Ref: %s", mpResp.ID, mpResp.Status, externalRef)

	return &PixPaymentResult{
		PaymentID: mpResp.ID,
		QRCodeB64: mpResp.PointOfInteraction.TransactionData.QRCodeBase64,
		QRCode:    mpResp.PointOfInteraction.TransactionData.QRCode,
		TicketURL: mpResp.PointOfInteraction.TransactionData.TicketURL,
		Status:    mpResp.Status,
	}, nil
}

// GetPaymentStatus consulta o status de um pagamento no Mercado Pago
func GetPaymentStatus(paymentID int64) (string, error) {
	accessToken := os.Getenv("MP_ACCESS_TOKEN")
	if accessToken == "" {
		return "", fmt.Errorf("MP_ACCESS_TOKEN não configurado")
	}

	url := fmt.Sprintf("https://api.mercadopago.com/v1/payments/%d", paymentID)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Status string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Status, nil
}

func generateIdempotencyKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// fallback
		for i := range b {
			b[i] = byte(mrand.Intn(256))
		}
	}
	return fmt.Sprintf("%x", b)
}
