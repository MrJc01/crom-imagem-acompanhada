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

func setupStatsDB(t *testing.T) func() {
	t.Helper()
	tmpDir := t.TempDir()
	os.Setenv("DATABASE_URL", filepath.Join(tmpDir, "stats.db")+"?_journal_mode=WAL")
	os.Setenv("STORAGE_PATH", filepath.Join(tmpDir, "storage"))
	os.Setenv("APP_SALT", "test_salt")
	database.InitDB()
	return func() {
		database.DB.Close()
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("STORAGE_PATH")
		os.Unsetenv("APP_SALT")
	}
}

func seedLinks(t *testing.T) {
	t.Helper()
	database.DB.Exec(`INSERT INTO links (id, original_url, max_views, total_views, unique_views, expires_at, payment_status, is_private, file_path)
		VALUES ('pub1', '', 0, 5, 3, datetime('now', '+7 days'), 'approved', 0, '/fake/img.png')`)
	database.DB.Exec(`INSERT INTO links (id, original_url, max_views, total_views, unique_views, expires_at, payment_status, is_private)
		VALUES ('priv1', '', 0, 10, 8, datetime('now', '+30 days'), 'approved', 1)`)
	database.DB.Exec(`INSERT INTO links (id, payment_status, is_private, password_hash, expires_at)
		VALUES ('pass1', 'approved', 1, 'a665a45920422f9d417e4867efdc4fb8a04a1f3fff1fa07e998e86f7f7a27ae3', datetime('now', '+7 days'))`)
	// password_hash = sha256("123")
	database.DB.Exec(`INSERT INTO access_logs (link_id, ip_hash, user_agent, country, city) VALUES ('pub1', 'h1', 'Chrome', 'BR', 'São Paulo')`)
	database.DB.Exec(`INSERT INTO access_logs (link_id, ip_hash, user_agent, country, city) VALUES ('pub1', 'h2', 'Firefox', 'US', 'New York')`)
	database.DB.Exec(`INSERT INTO access_logs (link_id, ip_hash, user_agent, country, city) VALUES ('pub1', 'h3', 'Safari', 'BR', 'São Paulo')`)
}

func TestPublicLinksHandler(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()
	seedLinks(t)

	req := httptest.NewRequest(http.MethodGet, "/api/public-links", nil)
	w := httptest.NewRecorder()
	PublicLinksHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d", w.Code)
	}

	var links []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &links)

	// Deve retornar apenas links públicos (is_private=0, approved, não expirado)
	if len(links) != 1 {
		t.Errorf("esperava 1 link público, obteve %d", len(links))
	}

	if len(links) > 0 && links[0]["id"] != "pub1" {
		t.Errorf("esperava id=pub1, obteve %v", links[0]["id"])
	}
}

func TestPublicLinksHandler_EmptyList(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/public-links", nil)
	w := httptest.NewRecorder()
	PublicLinksHandler(w, req)

	var links []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &links)

	if links == nil {
		t.Error("lista vazia deve retornar [] e não null")
	}
}

func TestLinkStatsHandler_MissingID(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/link-stats", nil)
	w := httptest.NewRecorder()
	LinkStatsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperava 400, obteve %d", w.Code)
	}
}

func TestLinkStatsHandler_InvalidPeriod(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/link-stats?id=pub1&period=99h", nil)
	w := httptest.NewRecorder()
	LinkStatsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperava 400 para período inválido, obteve %d", w.Code)
	}
}

func TestLinkStatsHandler_AllPeriods(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()
	seedLinks(t)

	periods := []string{"10m", "1h", "24h", "7d"}
	for _, p := range periods {
		t.Run("period="+p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/link-stats?id=pub1&period="+p, nil)
			w := httptest.NewRecorder()
			LinkStatsHandler(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("period=%s: esperava 200, obteve %d", p, w.Code)
			}

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)
			if resp["period"] != p {
				t.Errorf("esperava period=%s, obteve %v", p, resp["period"])
			}
		})
	}
}

func TestLinkStatsHandler_DefaultPeriod(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()
	seedLinks(t)

	req := httptest.NewRequest(http.MethodGet, "/api/link-stats?id=pub1", nil)
	w := httptest.NewRecorder()
	LinkStatsHandler(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["period"] != "24h" {
		t.Errorf("período padrão deveria ser 24h, obteve %v", resp["period"])
	}
}

func TestLinkGeoHandler_MissingID(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/link-geo", nil)
	w := httptest.NewRecorder()
	LinkGeoHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperava 400, obteve %d", w.Code)
	}
}

func TestLinkGeoHandler_WithData(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()
	seedLinks(t)

	req := httptest.NewRequest(http.MethodGet, "/api/link-geo?id=pub1", nil)
	w := httptest.NewRecorder()
	LinkGeoHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d", w.Code)
	}

	var data [][]interface{}
	json.Unmarshal(w.Body.Bytes(), &data)

	// Primeira linha deve ser header ["Country", "Views"]
	if len(data) < 1 {
		t.Fatal("resposta deve ter pelo menos o header")
	}
	if data[0][0] != "Country" {
		t.Errorf("primeira coluna do header esperada 'Country', obteve %v", data[0][0])
	}

	// Deve ter dados geo dos access_logs seedados (BR e US)
	if len(data) < 2 {
		t.Error("deveria ter ao menos 1 linha de dados geo além do header")
	}
}

func TestPrivateStatsHandler_MethodNotAllowed(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api/private-stats", nil)
	w := httptest.NewRecorder()
	PrivateStatsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("esperava 405, obteve %d", w.Code)
	}
}

func TestPrivateStatsHandler_NotFound(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()

	body := `{"id": "inexistente", "password": "abc"}`
	req := httptest.NewRequest(http.MethodPost, "/api/private-stats", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	PrivateStatsHandler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("esperava 404, obteve %d", w.Code)
	}
}

func TestPrivateStatsHandler_WrongPassword(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()
	seedLinks(t)

	body := `{"id": "pass1", "password": "wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/private-stats", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	PrivateStatsHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("esperava 401, obteve %d", w.Code)
	}
}

func TestPrivateStatsHandler_Success(t *testing.T) {
	cleanup := setupStatsDB(t)
	defer cleanup()
	seedLinks(t)

	body := `{"id": "pass1", "password": "123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/private-stats", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	PrivateStatsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d — body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["id"] != "pass1" {
		t.Errorf("esperava id=pass1, obteve %v", resp["id"])
	}
}
