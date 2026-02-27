package database

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB() {
	var err error
	dbPath := os.Getenv("DATABASE_URL")
	if dbPath == "" {
		dbPath = "./crom_vision.db?_journal_mode=WAL"
	}

	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Falha crítica ao abrir db: %v", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS links (
		id TEXT PRIMARY KEY,
		original_url TEXT,
		max_views INTEGER DEFAULT 0,
		total_views INTEGER DEFAULT 0,
		unique_views INTEGER DEFAULT 0,
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		tier TEXT DEFAULT 'free',
		email TEXT,
		payment_status TEXT DEFAULT 'pending',
		is_private BOOLEAN DEFAULT 0,
		password_hash TEXT,
		file_path TEXT,
		creator_ip TEXT
	);
	CREATE TABLE IF NOT EXISTS access_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		link_id TEXT,
		ip_hash TEXT,
		user_agent TEXT,
		country TEXT,
		city TEXT,
		accessed_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	if _, err := DB.Exec(schema); err != nil {
		log.Fatal("Falha ao migrar database local:", err)
	}

	migrations := []string{
		"ALTER TABLE links ADD COLUMN tier TEXT DEFAULT 'free'",
		"ALTER TABLE links ADD COLUMN email TEXT",
		"ALTER TABLE links ADD COLUMN payment_status TEXT DEFAULT 'pending'",
		"ALTER TABLE links ADD COLUMN is_private BOOLEAN DEFAULT 0",
		"ALTER TABLE links ADD COLUMN password_hash TEXT",
		"ALTER TABLE links ADD COLUMN file_path TEXT",
		"ALTER TABLE links ADD COLUMN creator_ip TEXT",
		// v5 — Mercado Pago PIX
		"ALTER TABLE links ADD COLUMN price REAL DEFAULT 0",
		"ALTER TABLE links ADD COLUMN mp_payment_id TEXT",
		"ALTER TABLE links ADD COLUMN mp_qr_code TEXT",
		"ALTER TABLE links ADD COLUMN mp_qr_base64 TEXT",
		"ALTER TABLE links ADD COLUMN mp_ticket_url TEXT",
	}
	for _, q := range migrations {
		DB.Exec(q) 
	}

	storagePath := os.Getenv("STORAGE_PATH")
	if storagePath == "" {
		storagePath = "./storage"
	}
	os.MkdirAll(storagePath, os.ModePerm)

	log.Println("Database local iniciada / v4 Snipboard Híbrida")
}
