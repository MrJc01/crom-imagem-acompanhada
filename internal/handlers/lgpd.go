package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"crom-vision/internal/database"
)

// LGPDConsultarHandler permite ao titular consultar quais dados existem sobre ele.
// POST /api/lgpd/consultar — body: {"email": "usuario@email.com"}
func LGPDConsultarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "Email obrigatório", http.StatusBadRequest)
		return
	}

	// Buscar links criados por este email
	rows, err := database.DB.Query(`
		SELECT id, tier, payment_status, created_at, expires_at 
		FROM links 
		WHERE email = ?`, req.Email)
	if err != nil {
		http.Error(w, "Erro interno", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var dados []map[string]interface{}
	for rows.Next() {
		var id, tier, status string
		var createdAt, expiresAt interface{}
		rows.Scan(&id, &tier, &status, &createdAt, &expiresAt)
		dados = append(dados, map[string]interface{}{
			"id":             id,
			"tier":           tier,
			"payment_status": status,
			"created_at":     createdAt,
			"expires_at":     expiresAt,
		})
	}
	if dados == nil {
		dados = []map[string]interface{}{}
	}

	// Contar logs de acesso associados
	var totalLogs int
	for _, d := range dados {
		var count int
		database.DB.QueryRow("SELECT COUNT(*) FROM access_logs WHERE link_id = ?", d["id"]).Scan(&count)
		totalLogs += count
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"email":            req.Email,
		"links_encontrados": len(dados),
		"total_logs_acesso": totalLogs,
		"dados":            dados,
		"mensagem":         "Para solicitar exclusão, use DELETE /api/lgpd/apagar com este mesmo email.",
	})
}

// LGPDApagarHandler permite ao titular solicitar exclusão de todos os seus dados.
// DELETE /api/lgpd/apagar — body: {"email": "usuario@email.com"}
func LGPDApagarHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method Not Allowed — Use DELETE", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, "Email obrigatório", http.StatusBadRequest)
		return
	}

	// 1. Coletar file_paths para apagar arquivos físicos
	rows, err := database.DB.Query(`SELECT id, file_path FROM links WHERE email = ?`, req.Email)
	if err != nil {
		http.Error(w, "Erro interno", http.StatusInternalServerError)
		return
	}

	var linkIDs []string
	var filePaths []string
	for rows.Next() {
		var id string
		var fp *string
		rows.Scan(&id, &fp)
		linkIDs = append(linkIDs, id)
		if fp != nil && *fp != "" {
			filePaths = append(filePaths, *fp)
		}
	}
	rows.Close()

	if len(linkIDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"mensagem":     "Nenhum dado encontrado para este email.",
			"links_removidos": 0,
		})
		return
	}

	// 2. Apagar arquivos físicos
	for _, fp := range filePaths {
		storagePath := os.Getenv("STORAGE_PATH")
		if storagePath == "" {
			storagePath = "./storage"
		}
		// Se file_path é só basename, reconstrói caminho
		fullPath := fp
		if len(fp) > 0 && fp[0] != '/' && fp[0] != '.' {
			fullPath = storagePath + "/" + fp
		}
		os.Remove(fullPath)
	}

	// 3. Apagar access_logs
	var totalLogs int64
	for _, lid := range linkIDs {
		res, _ := database.DB.Exec("DELETE FROM access_logs WHERE link_id = ?", lid)
		if res != nil {
			n, _ := res.RowsAffected()
			totalLogs += n
		}
	}

	// 4. Apagar links
	res, _ := database.DB.Exec("DELETE FROM links WHERE email = ?", req.Email)
	linksRemoved, _ := res.RowsAffected()

	// 5. Log de auditoria
	h := sha256.New()
	h.Write([]byte(req.Email))
	emailHash := hex.EncodeToString(h.Sum(nil))[:12]
	log.Printf("[LGPD] Exclusão solicitada: email_hash=%s, links=%d, logs=%d, arquivos=%d",
		emailHash, linksRemoved, totalLogs, len(filePaths))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"mensagem":           "Todos os dados associados foram removidos permanentemente.",
		"links_removidos":    linksRemoved,
		"logs_removidos":     totalLogs,
		"arquivos_removidos": len(filePaths),
	})
}
