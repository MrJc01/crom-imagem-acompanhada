package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"crom-vision/internal/database"
)

func setupTestDB(t *testing.T) func() {
	t.Helper()
	tmpDir := t.TempDir()
	os.Setenv("DATABASE_URL", filepath.Join(tmpDir, "test.db")+"?_journal_mode=WAL")
	os.Setenv("STORAGE_PATH", filepath.Join(tmpDir, "storage"))
	os.Setenv("APP_MODE", "free")
	os.Setenv("BASE_URL", "http://test.local")
	os.Setenv("APP_SALT", "test_salt_123")
	database.InitDB()

	return func() {
		database.DB.Close()
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("STORAGE_PATH")
		os.Unsetenv("APP_MODE")
		os.Unsetenv("BASE_URL")
		os.Unsetenv("APP_SALT")
	}
}

func TestCheckoutHandler_MethodNotAllowed(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/checkout", nil)
	w := httptest.NewRecorder()
	CheckoutHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("esperava 405, obteve %d", w.Code)
	}
}

func TestCheckoutHandler_FreeMode_SemImagem(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("tier", "7d")
	writer.WriteField("email", "test@test.com")
	writer.WriteField("max_views", "100")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/checkout", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	CheckoutHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("JSON inválido: %v", err)
	}

	if resp["payment_status"] != "approved" {
		t.Errorf("esperava payment_status=approved, obteve %v", resp["payment_status"])
	}
	if resp["is_private"] != false {
		t.Errorf("esperava is_private=false, obteve %v", resp["is_private"])
	}
	if resp["id"] == nil || resp["id"] == "" {
		t.Error("ID deve ser gerado")
	}
	if resp["pixel_url"] == nil || resp["pixel_url"] == "" {
		t.Error("pixel_url deve ser retornado")
	}
}

func TestCheckoutHandler_FreeMode_ComImagem(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("tier", "3d")
	writer.WriteField("email", "")
	writer.WriteField("max_views", "0")

	// Criar fake PNG (header válido)
	part, _ := writer.CreateFormFile("image", "test.png")
	// PNG header mínimo: 8 bytes magic + IHDR chunk
	pngHeader := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG magic
		0x00, 0x00, 0x00, 0x0D, // IHDR chunk length
		0x49, 0x48, 0x44, 0x52, // IHDR
		0x00, 0x00, 0x00, 0x01, // width = 1
		0x00, 0x00, 0x00, 0x01, // height = 1
		0x08, 0x02, // bit depth + color type
		0x00, 0x00, 0x00, // compression, filter, interlace
	}
	part.Write(pngHeader)
	// Padding para preencher o buffer de detecção
	part.Write(make([]byte, 512))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/checkout", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	CheckoutHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["payment_status"] != "approved" {
		t.Errorf("esperava approved, obteve %v", resp["payment_status"])
	}
	if resp["preview_url"] == nil || resp["preview_url"] == "" {
		t.Error("preview_url deve ser retornada com imagem")
	}
}

func TestCheckoutHandler_InvalidFileFormat(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("tier", "1d")

	// Arquivo de texto — NÃO é imagem válida
	part, _ := writer.CreateFormFile("image", "hack.txt")
	part.Write([]byte("#!/bin/bash\nrm -rf /\necho 'hacked'"))
	// Padding
	part.Write(make([]byte, 512))
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/checkout", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	CheckoutHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperava 400 para arquivo inválido, obteve %d — body: %s", w.Code, w.Body.String())
	}
}

func TestCheckoutHandler_SaaS_PendingPayment(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	os.Setenv("APP_MODE", "saas")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("tier", "7d") // R$5 — plano pago
	writer.WriteField("email", "payer@test.com")
	writer.WriteField("max_views", "500")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/checkout", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	CheckoutHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["payment_status"] != "pending" {
		t.Errorf("modo SaaS com tier pago deve ser pending, obteve: %v", resp["payment_status"])
	}
	if resp["is_private"] != true {
		t.Errorf("modo SaaS com tier pago deve ser private, obteve: %v", resp["is_private"])
	}
	if resp["temp_password"] == nil || resp["temp_password"] == "" {
		t.Error("deve gerar senha temporária no modo SaaS pago")
	}
}

func TestCheckoutHandler_FreeUploadLimitPerIP(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	os.Setenv("FREE_UPLOADS_IP_LIMIT", "2")
	defer os.Unsetenv("FREE_UPLOADS_IP_LIMIT")

	// Fazer 2 uploads — ambos devem funcionar
	for i := 0; i < 2; i++ {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.WriteField("tier", "1d")
		writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/checkout", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		CheckoutHandler(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("upload %d falhou: %d", i+1, w.Code)
		}
	}

	// 3º upload deve ser bloqueado (429)
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("tier", "1d")
	writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/checkout", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	CheckoutHandler(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("esperava 429 após limite, obteve %d", w.Code)
	}
}

func TestCheckoutHandler_ExpiresAtFuture(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("tier", "7d")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/checkout", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	CheckoutHandler(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	expStr, ok := resp["expires_at"].(string)
	if !ok || expStr == "" {
		t.Error("expires_at deve ser retornado como string não-vazia")
	}
}

// Helper: cria link de teste diretamente no DB
func insertTestLink(t *testing.T, id, originalURL, paymentStatus, filePath string, maxViews, totalViews int, isPrivate bool) {
	t.Helper()
	_, err := database.DB.Exec(`
		INSERT INTO links (id, original_url, max_views, total_views, expires_at, payment_status, is_private, file_path, password_hash)
		VALUES (?, ?, ?, ?, datetime('now', '+7 days'), ?, ?, ?, '')`,
		id, originalURL, maxViews, totalViews, paymentStatus, isPrivate, filePath)
	if err != nil {
		t.Fatalf("falha ao inserir link de teste: %v", err)
	}
}

func TestImageHandler_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/i/inexistente", nil)
	w := httptest.NewRecorder()
	ImageHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperava 200 (GIF transparente), obteve %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/gif" {
		t.Errorf("Content-Type: esperava image/gif, obteve %s", ct)
	}
}

func TestImageHandler_PaymentPending(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	insertTestLink(t, "test_pending", "", "pending", "", 0, 0, true)

	req := httptest.NewRequest(http.MethodGet, "/i/test_pending", nil)
	w := httptest.NewRecorder()
	ImageHandler(w, req)

	if w.Header().Get("X-Crom-Status") != "Payment-Pending" {
		t.Errorf("esperava header X-Crom-Status=Payment-Pending, obteve %q", w.Header().Get("X-Crom-Status"))
	}
}

func TestImageHandler_Redirect(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	insertTestLink(t, "test_redirect", "https://crom.run", "approved", "", 0, 0, false)

	req := httptest.NewRequest(http.MethodGet, "/i/test_redirect", nil)
	w := httptest.NewRecorder()
	ImageHandler(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("esperava 302 redirect, obteve %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "https://crom.run" {
		t.Errorf("esperava redirect para https://crom.run, obteve %q", loc)
	}
}

func TestImageHandler_ServeFile(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Criar arquivo de imagem temporário
	tmpFile := filepath.Join(t.TempDir(), "test.png")
	os.WriteFile(tmpFile, []byte("fake-image-data"), 0644)

	insertTestLink(t, "test_file", "", "approved", tmpFile, 0, 0, false)

	req := httptest.NewRequest(http.MethodGet, "/i/test_file", nil)
	w := httptest.NewRecorder()
	ImageHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperava 200, obteve %d", w.Code)
	}
	bodyBytes, _ := io.ReadAll(w.Result().Body)
	if string(bodyBytes) != "fake-image-data" {
		t.Errorf("corpo da resposta inesperado: %q", string(bodyBytes))
	}
}

func TestImageHandler_ViewLimitReached(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	insertTestLink(t, "test_maxed", "", "approved", "", 10, 10, false)

	req := httptest.NewRequest(http.MethodGet, "/i/test_maxed", nil)
	w := httptest.NewRecorder()
	ImageHandler(w, req)

	if w.Header().Get("X-Crom-Status") != "Limit-Reached" {
		t.Errorf("esperava X-Crom-Status=Limit-Reached, obteve %q", w.Header().Get("X-Crom-Status"))
	}
}

func TestPreviewHandler_NotFound(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/p/naoexiste", nil)
	w := httptest.NewRecorder()
	PreviewHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("esperava 404, obteve %d", w.Code)
	}
}

func TestPreviewHandler_PaymentPending(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	tmpFile := filepath.Join(t.TempDir(), "img.png")
	os.WriteFile(tmpFile, []byte("data"), 0644)
	insertTestLink(t, "test_pprev", "", "pending", tmpFile, 0, 0, true)

	req := httptest.NewRequest(http.MethodGet, "/p/test_pprev", nil)
	w := httptest.NewRecorder()
	PreviewHandler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("esperava 403, obteve %d", w.Code)
	}
}

func TestPreviewHandler_Private(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	tmpFile := filepath.Join(t.TempDir(), "img.png")
	os.WriteFile(tmpFile, []byte("data"), 0644)
	insertTestLink(t, "test_priv", "", "approved", tmpFile, 0, 0, true)

	req := httptest.NewRequest(http.MethodGet, "/p/test_priv", nil)
	w := httptest.NewRecorder()
	PreviewHandler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("esperava 403 para privado, obteve %d", w.Code)
	}
}

func TestPreviewHandler_Success(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()
	tmpFile := filepath.Join(t.TempDir(), "img.png")
	os.WriteFile(tmpFile, []byte("png-data-here"), 0644)
	insertTestLink(t, "test_preview", "", "approved", tmpFile, 0, 0, false)

	req := httptest.NewRequest(http.MethodGet, "/p/test_preview", nil)
	w := httptest.NewRecorder()
	PreviewHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("esperava 200, obteve %d", w.Code)
	}
}
