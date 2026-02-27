package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"strings"
	"time"

	"crom-vision/internal/database"
	"crom-vision/internal/utils"
)

func ImageHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/i/"):]
	if id == "" {
		http.Error(w, "ID missing", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")

	var originalURL sql.NullString
	var maxViews, totalViews int
	var expiresAt sql.NullTime
	var paymentStatus string
	var filePath sql.NullString

	err := database.DB.QueryRow("SELECT original_url, max_views, total_views, expires_at, payment_status, file_path FROM links WHERE id = ?", id).
		Scan(&originalURL, &maxViews, &totalViews, &expiresAt, &paymentStatus, &filePath)

	if err != nil {
		w.Header().Set("Content-Type", "image/gif")
		w.Write(utils.TransparentGif)
		return
	}

	if paymentStatus != "approved" {
		w.Header().Set("X-Crom-Status", "Payment-Pending")
		w.Header().Set("Content-Type", "image/gif")
		w.Write(utils.TransparentGif)
		return
	}

	if expiresAt.Valid && time.Now().After(expiresAt.Time) {
		w.Header().Set("X-Crom-Status", "Expired-Time")
		w.Header().Set("Content-Type", "image/gif")
		w.Write(utils.TransparentGif)
		return
	}
	if maxViews > 0 && totalViews >= maxViews {
		w.Header().Set("X-Crom-Status", "Limit-Reached")
		w.Header().Set("Content-Type", "image/gif")
		w.Write(utils.TransparentGif)
		return
	}

	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = forwarded
	}

	// GeoIP lookup — só se GEO_TRACKING_ENABLED estiver ativo
	var country, city string
	if strings.ToLower(os.Getenv("GEO_TRACKING_ENABLED")) != "false" {
		country, city = utils.LookupGeoIP(ip)
	}

	ua := r.UserAgent()
	// LGPD: truncar User-Agent para evitar fingerprinting excessivo
	if len(ua) > 120 {
		ua = ua[:120]
	}
	fingerprintHash := utils.ComposeFingerprintHash(ip, ua)
	fingerprintKey := id + "::" + fingerprintHash
	isUnique := utils.IsUniqueAccess(fingerprintKey)

	go func(linkID, fHash, uaStr, ctry, cty string, uniq bool) {
		if uniq {
			database.DB.Exec("UPDATE links SET total_views = total_views + 1, unique_views = unique_views + 1 WHERE id = ?", linkID)
		} else {
			database.DB.Exec("UPDATE links SET total_views = total_views + 1 WHERE id = ?", linkID)
		}
		database.DB.Exec("INSERT INTO access_logs (link_id, ip_hash, user_agent, country, city) VALUES (?, ?, ?, ?, ?)",
			linkID, fHash, uaStr, ctry, cty)
	}(id, fingerprintHash, ua, country, city, isUnique)

	if originalURL.Valid && originalURL.String != "" {
		http.Redirect(w, r, originalURL.String, http.StatusFound)
		return
	}

	if filePath.Valid && filePath.String != "" {
		w.Header().Del("Content-Type") 
		http.ServeFile(w, r, filePath.String)
		return
	}

	w.Header().Set("Content-Type", "image/gif")
	w.Write(utils.TransparentGif)
}

func PreviewHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/p/"):]
	if id == "" {
		http.Error(w, "ID missing", http.StatusBadRequest)
		return
	}

	var filePath sql.NullString
	var isPrivate bool
	var paymentStatus string
	err := database.DB.QueryRow("SELECT file_path, is_private, payment_status FROM links WHERE id = ?", id).Scan(&filePath, &isPrivate, &paymentStatus)
	
	if err != nil || !filePath.Valid || filePath.String == "" {
		http.NotFound(w, r)
		return
	}

	if paymentStatus != "approved" {
		http.Error(w, "Payment Pending", http.StatusForbidden)
		return
	}

	if isPrivate {
		http.Error(w, "Private Asset", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, filePath.String)
}
