package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"crom-vision/internal/database"
)

func PublicLinksHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := database.DB.Query(`
		SELECT id, original_url, max_views, total_views, unique_views, expires_at, file_path 
		FROM links 
		WHERE is_private = 0 AND payment_status = 'approved' AND (expires_at > CURRENT_TIMESTAMP OR expires_at IS NULL)
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var links []map[string]interface{}
	for rows.Next() {
		var id string
		var orig sql.NullString
		var max, tot, uniq int
		var exp sql.NullTime
		var filePath sql.NullString

		rows.Scan(&id, &orig, &max, &tot, &uniq, &exp, &filePath)
		
		hasImage := false
		if filePath.Valid && filePath.String != "" {
			hasImage = true
		}

		links = append(links, map[string]interface{}{
			"id":           id,
			"original_url": orig.String,
			"max_views":    max,
			"total_views":  tot,
			"unique_views": uniq,
			"expires_at":   exp.Time,
			"has_image":    hasImage,
		})
	}
	if links == nil { links = []map[string]interface{}{} }

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

func PrivateStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID       string `json:"id"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid Payload", http.StatusBadRequest)
		return
	}

	var dbPassHash string
	var orig sql.NullString
	var max, tot, uniq int
	var exp sql.NullTime
	var paymentStatus string

	err := database.DB.QueryRow(`
		SELECT original_url, max_views, total_views, unique_views, expires_at, payment_status, password_hash
		FROM links WHERE id = ?`, req.ID).Scan(&orig, &max, &tot, &uniq, &exp, &paymentStatus, &dbPassHash)

	if err != nil {
		http.Error(w, "Asset not found", http.StatusNotFound)
		return
	}

	h := sha256.New()
	h.Write([]byte(req.Password))
	inpHash := hex.EncodeToString(h.Sum(nil))

	if inpHash != dbPassHash {
		http.Error(w, "Unauthorized (Invalid Password)", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":             req.ID,
		"original_url":   orig.String,
		"max_views":      max,
		"total_views":    tot,
		"unique_views":   uniq,
		"expires_at":     exp.Time,
		"payment_status": paymentStatus,
	})
}

func LinkStatsHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	period := r.URL.Query().Get("period")
	if id == "" {
		http.Error(w, "ID missing", http.StatusBadRequest)
		return
	}
	if period == "" {
		period = "24h"
	}

	// Permissão livre? No momento Dashboard lateral é public em modo FREE ou mostra tudo publico por ID 
	// (Num SaaS real seria bom pedir a senha mágica, por conveniência vitrine/dash está aberto)

	var query string
	var timeMod string

	switch period {
	case "10m":
		// Dados dos últimos 10 minutos agrupados por minuto exato.
		timeMod = "-10 minutes"
		query = `
			SELECT strftime('%H:%M', accessed_at) as bucket, COUNT(*) 
			FROM access_logs 
			WHERE link_id = ? AND accessed_at >= datetime('now', ?) 
			GROUP BY bucket ORDER BY bucket ASC`
	case "1h":
		// Última 1 hora agrupada a cada 10 minutos - simplificado para minuto a minuto na visualização
		timeMod = "-1 hour"
		query = `
			SELECT strftime('%H:%M', accessed_at, '-' || (CAST(strftime('%M', accessed_at) AS INTEGER) % 10) || ' minutes') as bucket, COUNT(*) 
			FROM access_logs 
			WHERE link_id = ? AND accessed_at >= datetime('now', ?) 
			GROUP BY bucket ORDER BY bucket ASC`
	case "24h":
		// Últimas 24 horas agrupadas por hora cheia
		timeMod = "-24 hours"
		query = `
			SELECT strftime('%Y-%m-%d %H:00', accessed_at) as bucket, COUNT(*) 
			FROM access_logs 
			WHERE link_id = ? AND accessed_at >= datetime('now', ?) 
			GROUP BY bucket ORDER BY bucket ASC`
	case "7d":
		// Últimos 7 dias agrupados por dia
		timeMod = "-7 days"
		query = `
			SELECT strftime('%Y-%m-%d', accessed_at) as bucket, COUNT(*) 
			FROM access_logs 
			WHERE link_id = ? AND accessed_at >= datetime('now', ?) 
			GROUP BY bucket ORDER BY bucket ASC`
	default:
		http.Error(w, "Invalid period", http.StatusBadRequest)
		return
	}

	rows, err := database.DB.Query(query, id, timeMod)
	if err != nil {
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var labels []string
	var data []int

	for rows.Next() {
		var bucket string
		var count int
		rows.Scan(&bucket, &count)
		labels = append(labels, bucket)
		data = append(data, count)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"labels": labels,
		"data":   data,
		"period": period,
	})
}

func LinkGeoHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID missing", http.StatusBadRequest)
		return
	}

	// Se geo tracking desativado, retorna vazio
	if strings.ToLower(os.Getenv("GEO_TRACKING_ENABLED")) == "false" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([][]interface{}{{"Country", "Views"}})
		return
	}

	rows, err := database.DB.Query(`
		SELECT country, COUNT(*) 
		FROM access_logs 
		WHERE link_id = ? 
		GROUP BY country ORDER BY COUNT(*) DESC`, id)
	
	if err != nil {
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var data [][]interface{}
	// First row represents the columns for Google GeoChart
	data = append(data, []interface{}{"Country", "Views"})

	for rows.Next() {
		var country string
		var count int
		rows.Scan(&country, &count)
		data = append(data, []interface{}{country, count})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
