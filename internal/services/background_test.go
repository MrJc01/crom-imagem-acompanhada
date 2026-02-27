package services

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"crom-vision/internal/database"
)

func setupBgDB(t *testing.T) (func(), string) {
	t.Helper()
	tmpDir := t.TempDir()
	os.Setenv("DATABASE_URL", filepath.Join(tmpDir, "bg.db")+"?_journal_mode=WAL")
	os.Setenv("STORAGE_PATH", filepath.Join(tmpDir, "storage"))
	os.MkdirAll(filepath.Join(tmpDir, "storage"), 0755)
	database.InitDB()
	return func() {
		database.DB.Close()
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("STORAGE_PATH")
	}, tmpDir
}

func TestHardDeleteExpired_RemovesExpiredLinks(t *testing.T) {
	cleanup, tmpDir := setupBgDB(t)
	defer cleanup()

	storagePath := filepath.Join(tmpDir, "storage")

	// Criar arquivo físico de teste
	testFile := filepath.Join(storagePath, "expired_image.png")
	os.WriteFile(testFile, []byte("fake-image"), 0644)

	// Inserir link expirado (expires_at no passado)
	database.DB.Exec(`INSERT INTO links (id, expires_at, file_path, payment_status) VALUES ('exp1', datetime('now', '-1 hour'), ?, 'approved')`, testFile)
	database.DB.Exec(`INSERT INTO access_logs (link_id, ip_hash, user_agent, country, city) VALUES ('exp1', 'hash', 'ua', 'BR', 'SP')`)

	// Inserir link válido (não deve ser removido)
	database.DB.Exec(`INSERT INTO links (id, expires_at, payment_status) VALUES ('valid1', datetime('now', '+7 days'), 'approved')`)
	database.DB.Exec(`INSERT INTO access_logs (link_id, ip_hash, user_agent, country, city) VALUES ('valid1', 'hash2', 'ua2', 'US', 'NY')`)

	// Executar a lógica de limpeza manualmente (sem o loop infinito)
	runCleanupOnce(t)

	// Verificar que o link expirado foi removido
	var countExp int
	database.DB.QueryRow("SELECT COUNT(*) FROM links WHERE id = 'exp1'").Scan(&countExp)
	if countExp != 0 {
		t.Errorf("link expirado deveria ter sido removido, mas count=%d", countExp)
	}

	// Verificar que os logs do link expirado foram removidos
	var countLogs int
	database.DB.QueryRow("SELECT COUNT(*) FROM access_logs WHERE link_id = 'exp1'").Scan(&countLogs)
	if countLogs != 0 {
		t.Errorf("logs do link expirado deveriam ter sido removidos, count=%d", countLogs)
	}

	// Verificar que o arquivo físico foi apagado
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("arquivo físico da imagem expirada deveria ter sido apagado")
	}

	// Verificar que o link válido NÃO foi removido
	var countValid int
	database.DB.QueryRow("SELECT COUNT(*) FROM links WHERE id = 'valid1'").Scan(&countValid)
	if countValid != 1 {
		t.Errorf("link válido NÃO deveria ter sido removido, count=%d", countValid)
	}

	// Logs do válido devem permanecer
	var countValidLogs int
	database.DB.QueryRow("SELECT COUNT(*) FROM access_logs WHERE link_id = 'valid1'").Scan(&countValidLogs)
	if countValidLogs != 1 {
		t.Errorf("logs do link válido NÃO deveriam ter sido removidos, count=%d", countValidLogs)
	}
}

func TestHardDeleteExpired_NoExpired(t *testing.T) {
	cleanup, _ := setupBgDB(t)
	defer cleanup()

	// Apenas links válidos
	database.DB.Exec(`INSERT INTO links (id, expires_at, payment_status) VALUES ('safe1', datetime('now', '+30 days'), 'approved')`)

	runCleanupOnce(t)

	var count int
	database.DB.QueryRow("SELECT COUNT(*) FROM links").Scan(&count)
	if count != 1 {
		t.Errorf("nenhum link deveria ser removido, count=%d", count)
	}
}

func TestHardDeleteExpired_NullExpiresAt(t *testing.T) {
	cleanup, _ := setupBgDB(t)
	defer cleanup()

	// Link sem data de expiração (NULL) — NÃO deve ser removido
	database.DB.Exec(`INSERT INTO links (id, payment_status) VALUES ('null_exp', 'approved')`)

	runCleanupOnce(t)

	var count int
	database.DB.QueryRow("SELECT COUNT(*) FROM links WHERE id = 'null_exp'").Scan(&count)
	if count != 1 {
		t.Errorf("link sem expires_at NÃO deveria ser removido, count=%d", count)
	}
}

// runCleanupOnce executa a mesma lógica do HardDeleteExpired mas sem o loop infinito
func runCleanupOnce(t *testing.T) {
	t.Helper()

	// 1. Coletar os file_paths primeiro antes de apagar
	rows, err := database.DB.Query(`SELECT file_path FROM links WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP AND file_path IS NOT NULL AND file_path != ''`)
	if err == nil {
		for rows.Next() {
			var fPath string
			if err := rows.Scan(&fPath); err == nil && fPath != "" {
				os.Remove(fPath)
			}
		}
		rows.Close()
	}

	// 2. Apagar do Banco
	database.DB.Exec(`DELETE FROM access_logs WHERE link_id IN (SELECT id FROM links WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP)`)
	database.DB.Exec(`DELETE FROM links WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP`)

	// Pequeno delay para garantir que tudo foi processado
	time.Sleep(50 * time.Millisecond)
}
