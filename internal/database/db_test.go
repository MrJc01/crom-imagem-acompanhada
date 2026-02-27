package database

import (
	"os"
	"testing"
)

func TestInitDB(t *testing.T) {
	// Usar banco temporário para não interferir com produção
	tmpFile := t.TempDir() + "/test.db"
	os.Setenv("DATABASE_URL", tmpFile+"?_journal_mode=WAL")
	os.Setenv("STORAGE_PATH", t.TempDir()+"/storage")
	defer os.Unsetenv("DATABASE_URL")
	defer os.Unsetenv("STORAGE_PATH")

	t.Run("deve inicializar sem pânico", func(t *testing.T) {
		InitDB()
		defer DB.Close()

		if DB == nil {
			t.Fatal("DB é nil após InitDB()")
		}
	})

	t.Run("tabela links deve existir", func(t *testing.T) {
		InitDB()
		defer DB.Close()

		var count int
		err := DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='links'").Scan(&count)
		if err != nil {
			t.Fatalf("erro ao verificar tabela links: %v", err)
		}
		if count != 1 {
			t.Errorf("tabela links não encontrada — count=%d", count)
		}
	})

	t.Run("tabela access_logs deve existir", func(t *testing.T) {
		InitDB()
		defer DB.Close()

		var count int
		err := DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='access_logs'").Scan(&count)
		if err != nil {
			t.Fatalf("erro ao verificar tabela access_logs: %v", err)
		}
		if count != 1 {
			t.Errorf("tabela access_logs não encontrada — count=%d", count)
		}
	})

	t.Run("deve criar diretório de storage", func(t *testing.T) {
		storagePath := t.TempDir() + "/new_storage"
		os.Setenv("STORAGE_PATH", storagePath)
		InitDB()
		defer DB.Close()

		if _, err := os.Stat(storagePath); os.IsNotExist(err) {
			t.Errorf("diretório de storage %q não foi criado", storagePath)
		}
	})

	t.Run("schema de links deve ter todas as colunas", func(t *testing.T) {
		InitDB()
		defer DB.Close()

		expectedCols := []string{
			"id", "original_url", "max_views", "total_views", "unique_views",
			"expires_at", "created_at", "tier", "email", "payment_status",
			"is_private", "password_hash", "file_path", "creator_ip",
		}
		for _, col := range expectedCols {
			row := DB.QueryRow("SELECT " + col + " FROM links LIMIT 0")
			if err := row.Err(); err != nil {
				t.Errorf("coluna %q não encontrada na tabela links: %v", col, err)
			}
		}
	})

	t.Run("schema de access_logs deve ter todas as colunas", func(t *testing.T) {
		InitDB()
		defer DB.Close()

		expectedCols := []string{
			"id", "link_id", "ip_hash", "user_agent", "country", "city", "accessed_at",
		}
		for _, col := range expectedCols {
			row := DB.QueryRow("SELECT " + col + " FROM access_logs LIMIT 0")
			if err := row.Err(); err != nil {
				t.Errorf("coluna %q não encontrada na tabela access_logs: %v", col, err)
			}
		}
	})

	t.Run("deve poder inserir e buscar um link", func(t *testing.T) {
		InitDB()
		defer DB.Close()

		_, err := DB.Exec(`INSERT INTO links (id, original_url, max_views, tier, payment_status) VALUES (?, ?, ?, ?, ?)`,
			"test_001", "https://example.com", 100, "free", "approved")
		if err != nil {
			t.Fatalf("erro ao inserir: %v", err)
		}

		var id, tier, status string
		err = DB.QueryRow("SELECT id, tier, payment_status FROM links WHERE id = ?", "test_001").Scan(&id, &tier, &status)
		if err != nil {
			t.Fatalf("erro ao buscar: %v", err)
		}
		if id != "test_001" || tier != "free" || status != "approved" {
			t.Errorf("dados incorretos: id=%s, tier=%s, status=%s", id, tier, status)
		}
	})
}
