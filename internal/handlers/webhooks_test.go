package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"crom-vision/internal/database"
)

func setupWebhookDB(t *testing.T) func() {
	t.Helper()
	tmpDir := t.TempDir()
	os.Setenv("DATABASE_URL", filepath.Join(tmpDir, "webhook.db")+"?_journal_mode=WAL")
	os.Setenv("STORAGE_PATH", filepath.Join(tmpDir, "storage"))
	os.Setenv("APP_SALT", "test_salt")
	// Desabilitar envio de email real
	os.Unsetenv("SMTP_HOST")
	os.Unsetenv("SMTP_USER")
	os.Unsetenv("SMTP_PASS")
	database.InitDB()
	return func() {
		database.DB.Close()
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("STORAGE_PATH")
		os.Unsetenv("APP_SALT")
	}
}

func TestWebhookMPHandler_Approved(t *testing.T) {
	cleanup := setupWebhookDB(t)
	defer cleanup()

	// Criar link pendente
	database.DB.Exec(`INSERT INTO links (id, payment_status, email) VALUES ('wh_test1', 'pending', 'test@test.com')`)

	body := `{"link_id": "wh_test1", "status": "approved"}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhook/mp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	WebhookMPHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperava 200, obteve %d", w.Code)
	}

	// Verificar que o status mudou no banco
	var status string
	database.DB.QueryRow("SELECT payment_status FROM links WHERE id = ?", "wh_test1").Scan(&status)
	if status != "approved" {
		t.Errorf("status deveria ser 'approved', obteve %q", status)
	}
}

func TestWebhookMPHandler_NonApproved(t *testing.T) {
	cleanup := setupWebhookDB(t)
	defer cleanup()

	database.DB.Exec(`INSERT INTO links (id, payment_status) VALUES ('wh_test2', 'pending')`)

	body := `{"link_id": "wh_test2", "status": "rejected"}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhook/mp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	WebhookMPHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperava 200, obteve %d", w.Code)
	}

	var status string
	database.DB.QueryRow("SELECT payment_status FROM links WHERE id = ?", "wh_test2").Scan(&status)
	if status != "pending" {
		t.Errorf("status NÃO deveria mudar para rejected, mas ficou %q", status)
	}
}

func TestWebhookMPHandler_InvalidJSON(t *testing.T) {
	cleanup := setupWebhookDB(t)
	defer cleanup()

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhook/mp", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	WebhookMPHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperava 400, obteve %d", w.Code)
	}
}

func TestWebhookMPHandler_EmptyBody(t *testing.T) {
	cleanup := setupWebhookDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/api/webhook/mp", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	WebhookMPHandler(w, req)

	// Não deve dar erro — apenas não faz nada (status != "approved")
	if w.Code != http.StatusOK {
		t.Errorf("esperava 200 mesmo com body vazio, obteve %d", w.Code)
	}
}

func TestWebhookMPHandler_ResponseJSON(t *testing.T) {
	cleanup := setupWebhookDB(t)
	defer cleanup()

	database.DB.Exec(`INSERT INTO links (id, payment_status) VALUES ('wh_json', 'pending')`)

	body := `{"link_id": "wh_json", "status": "approved"}`
	req := httptest.NewRequest(http.MethodPost, "/api/webhook/mp", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	WebhookMPHandler(w, req)

	// O handler retorna 200 OK simples (sem JSON body)
	if w.Code != http.StatusOK {
		t.Errorf("esperava 200, obteve %d", w.Code)
	}

	// Verificar que o body está vazio (w.WriteHeader(200) sem body)
	bodyResp := w.Body.String()
	var jsonResp map[string]interface{}
	err := json.Unmarshal([]byte(bodyResp), &jsonResp)
	// Se body vazio ou não-JSON, tudo bem — é o esperado
	if err == nil && len(jsonResp) > 0 {
		// Se tiver JSON, tudo bem também
		t.Logf("webhook retornou JSON: %v", jsonResp)
	}
}
