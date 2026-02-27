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

func setupLGPDDB(t *testing.T) func() {
	t.Helper()
	tmpDir := t.TempDir()
	os.Setenv("DATABASE_URL", filepath.Join(tmpDir, "lgpd.db")+"?_journal_mode=WAL")
	os.Setenv("STORAGE_PATH", filepath.Join(tmpDir, "storage"))
	os.Setenv("APP_SALT", "test_salt")
	os.MkdirAll(filepath.Join(tmpDir, "storage"), 0755)
	database.InitDB()
	return func() {
		database.DB.Close()
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("STORAGE_PATH")
		os.Unsetenv("APP_SALT")
	}
}

func seedLGPDData(t *testing.T, tmpDir string) {
	t.Helper()
	// Criar arquivo físico
	imgPath := filepath.Join(tmpDir, "storage", "test_img.png")
	os.WriteFile(imgPath, []byte("fake-image"), 0644)

	database.DB.Exec(`INSERT INTO links (id, email, tier, payment_status, file_path, expires_at)
		VALUES ('lgpd_1', 'titular@test.com', '7d', 'approved', ?, datetime('now', '+7 days'))`, imgPath)
	database.DB.Exec(`INSERT INTO links (id, email, tier, payment_status, expires_at)
		VALUES ('lgpd_2', 'titular@test.com', '1d', 'approved', datetime('now', '+1 day'))`)
	database.DB.Exec(`INSERT INTO access_logs (link_id, ip_hash, user_agent, country, city) VALUES ('lgpd_1', 'h1', 'Chrome', 'BR', 'SP')`)
	database.DB.Exec(`INSERT INTO access_logs (link_id, ip_hash, user_agent, country, city) VALUES ('lgpd_1', 'h2', 'Firefox', 'US', 'NY')`)
	database.DB.Exec(`INSERT INTO access_logs (link_id, ip_hash, user_agent, country, city) VALUES ('lgpd_2', 'h3', 'Safari', 'BR', 'RJ')`)
}

func TestLGPDConsultar_Success(t *testing.T) {
	cleanup := setupLGPDDB(t)
	defer cleanup()
	seedLGPDData(t, t.TempDir())

	// Precisamos re-seed com o tmpDir correto
	database.DB.Exec(`INSERT INTO links (id, email, tier, payment_status) VALUES ('c1', 'user@test.com', '7d', 'approved')`)
	database.DB.Exec(`INSERT INTO access_logs (link_id, ip_hash, user_agent, country, city) VALUES ('c1', 'h', 'ua', 'BR', 'SP')`)

	body := `{"email": "user@test.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/lgpd/consultar", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	LGPDConsultarHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d — %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["links_encontrados"].(float64) != 1 {
		t.Errorf("esperava 1 link, obteve %v", resp["links_encontrados"])
	}
}

func TestLGPDConsultar_NotFound(t *testing.T) {
	cleanup := setupLGPDDB(t)
	defer cleanup()

	body := `{"email": "ninguem@nada.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/lgpd/consultar", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	LGPDConsultarHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["links_encontrados"].(float64) != 0 {
		t.Errorf("esperava 0 links, obteve %v", resp["links_encontrados"])
	}
}

func TestLGPDConsultar_MethodNotAllowed(t *testing.T) {
	cleanup := setupLGPDDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/lgpd/consultar", nil)
	w := httptest.NewRecorder()
	LGPDConsultarHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("esperava 405, obteve %d", w.Code)
	}
}

func TestLGPDConsultar_EmptyEmail(t *testing.T) {
	cleanup := setupLGPDDB(t)
	defer cleanup()

	body := `{"email": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/lgpd/consultar", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	LGPDConsultarHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperava 400, obteve %d", w.Code)
	}
}

func TestLGPDApagar_Success(t *testing.T) {
	cleanup := setupLGPDDB(t)
	defer cleanup()

	// Criar dados com arquivo
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "del_img.png")
	os.WriteFile(imgPath, []byte("data"), 0644)

	database.DB.Exec(`INSERT INTO links (id, email, file_path, payment_status) VALUES ('del1', 'apagar@test.com', ?, 'approved')`, imgPath)
	database.DB.Exec(`INSERT INTO access_logs (link_id, ip_hash, user_agent, country, city) VALUES ('del1', 'h', 'ua', 'BR', 'SP')`)

	body := `{"email": "apagar@test.com"}`
	req := httptest.NewRequest(http.MethodDelete, "/api/lgpd/apagar", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	LGPDApagarHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d — %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["links_removidos"].(float64) != 1 {
		t.Errorf("esperava 1 link removido, obteve %v", resp["links_removidos"])
	}

	// Verificar que foi realmente apagado do banco
	var count int
	database.DB.QueryRow("SELECT COUNT(*) FROM links WHERE email = 'apagar@test.com'").Scan(&count)
	if count != 0 {
		t.Errorf("link deveria ter sido removido do banco, count=%d", count)
	}

	// Verificar que arquivo foi apagado
	if _, err := os.Stat(imgPath); !os.IsNotExist(err) {
		t.Error("arquivo físico deveria ter sido apagado")
	}
}

func TestLGPDApagar_NoData(t *testing.T) {
	cleanup := setupLGPDDB(t)
	defer cleanup()

	body := `{"email": "naoexiste@test.com"}`
	req := httptest.NewRequest(http.MethodDelete, "/api/lgpd/apagar", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	LGPDApagarHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["links_removidos"].(float64) != 0 {
		t.Errorf("esperava 0, obteve %v", resp["links_removidos"])
	}
}

func TestLGPDApagar_MethodNotAllowed(t *testing.T) {
	cleanup := setupLGPDDB(t)
	defer cleanup()

	body := `{"email": "test@test.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/lgpd/apagar", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	LGPDApagarHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("esperava 405, obteve %d", w.Code)
	}
}
