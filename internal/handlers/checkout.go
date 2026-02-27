package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"crom-vision/internal/database"
	"crom-vision/internal/services"
	"crom-vision/internal/utils"
)

var tierPrices = map[string]float64{
	"1d":  0,
	"2d":  1,
	"3d":  2,
	"7d":  5,
	"1mo": 15,
	"1y":  50,
	"10y": 100,
}

var tierDurations = map[string]time.Duration{
	"1d":  24 * time.Hour,
	"2d":  48 * time.Hour,
	"3d":  72 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"1mo": 30 * 24 * time.Hour,
	"1y":  365 * 24 * time.Hour,
	"10y": 10 * 365 * 24 * time.Hour,
}

type CheckoutRequest struct {
	Email       string `json:"email"`
	OriginalURL string `json:"original_url"`
	Tier        string `json:"tier"` // 1d, 7d, 1mo... OU quantidade em dias caso FREE Mode
	MaxViews    int    `json:"max_views"`
}

func CheckoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		ip = forwarded
	}
	ipHash := utils.ComposeFingerprintHash(ip, "")

	appMode := os.Getenv("APP_MODE")
	if appMode == "" {
		appMode = "saas"
	}

	err := r.ParseMultipartForm(10 << 20) // 10MB limit
	if err != nil {
		http.Error(w, "Invalid Request Form", http.StatusBadRequest)
		return
	}

	tierReq := r.FormValue("tier")
	price := tierPrices[tierReq]

	isFreeRequest := false
	if appMode == "free" || price == 0 {
		isFreeRequest = true
	}

	limitStr := os.Getenv("FREE_UPLOADS_IP_LIMIT")
	if limitStr == "" {
		limitStr = "0"
	}
	limit, _ := strconv.Atoi(limitStr)

	if isFreeRequest && limit > 0 {
		var count int
		database.DB.QueryRow("SELECT COUNT(*) FROM links WHERE creator_ip = ? AND is_private = 0", ipHash).Scan(&count)
		if count >= limit {
			http.Error(w, "Limite de uploads gratuitos atingido para o seu IP.", http.StatusTooManyRequests)
			return
		}
	}

	email := r.FormValue("email")
	originalURL := r.FormValue("original_url")
	maxViews, _ := strconv.Atoi(r.FormValue("max_views"))

	var savedFilePath string
	file, header, errFile := r.FormFile("image")
	if errFile == nil {
		defer file.Close()
		
		buffer := make([]byte, 512)
		if n, err := file.Read(buffer); err == nil && n > 0 {
			contentType := http.DetectContentType(buffer[:n])
			if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/gif" {
				http.Error(w, "Invalid file format. Only JPG, PNG and GIF are allowed.", http.StatusBadRequest)
				return
			}
		} else {
             http.Error(w, "Failed to inspect file.", http.StatusInternalServerError)
             return
        }
        
        file.Seek(0, 0)
		
		storagePath := os.Getenv("STORAGE_PATH")
		if storagePath == "" {
			storagePath = "./storage"
		}
		
		ext := ""
		if header.Filename != "" {
			ext = filepath.Ext(header.Filename)
		}
		if ext == "" {
			ext = ".png"
		}

		fileName := utils.GenerateRandomString(12) + ext
		savedFilePath = filepath.Join(storagePath, fileName)

		out, err := os.Create(savedFilePath)
		if err == nil {
			defer out.Close()
			io.Copy(out, file)
		} else {
			savedFilePath = "" // failed to save
			log.Printf("[ERR] Falha ao tentar salvar arquivo: %v", err)
		}
	}

	duration := tierDurations[tierReq]

	if appMode == "free" {
		var customDays int
		if _, err := fmt.Sscanf(tierReq, "%dd", &customDays); err == nil {
			duration = time.Duration(customDays) * 24 * time.Hour
		} else {
			if duration == 0 {
				duration = 7 * 24 * time.Hour
			}
		}
		price = 0
	} else {
		if duration == 0 {
			http.Error(w, "Plano inválido no modo SaaS.", http.StatusBadRequest)
			return
		}
	}

	id := "c_" + utils.GenerateRandomString(4)
	expiresAt := time.Now().Add(duration)

	paymentStatus := "pending"
	isPrivate := true
	passwordHash := ""
	clearPassword := ""

	if appMode == "free" || price == 0 {
		paymentStatus = "approved"
		isPrivate = false
	} else {
		clearPassword = utils.GenerateRandomString(6)
		h := sha256.New()
		h.Write([]byte(clearPassword))
		passwordHash = hex.EncodeToString(h.Sum(nil))
	}

	// --- Mercado Pago PIX ---
	var mpPaymentID string
	var mpQRCode, mpQRBase64, mpTicketURL string

	if price > 0 && appMode != "free" {
		payerEmail := email
		if payerEmail == "" {
			payerEmail = "cliente@crom.run" // fallback para MP
		}
		description := fmt.Sprintf("Imagem Acompanhada — Tier %s — %s", tierReq, id)
		
		mpResult, err := services.CreatePixPayment(price, description, payerEmail, id)
		if err != nil {
			log.Printf("[MP ERR] Falha ao criar pagamento PIX: %v", err)
			// Continua sem PIX — frontend mostrará fallback
		} else {
			mpPaymentID = fmt.Sprintf("%d", mpResult.PaymentID)
			mpQRCode = mpResult.QRCode
			mpQRBase64 = mpResult.QRCodeB64
			mpTicketURL = mpResult.TicketURL
			paymentStatus = mpResult.Status // normalmente "pending"
		}
	}

	_, errDB := database.DB.Exec(`
		INSERT INTO links (id, original_url, max_views, expires_at, tier, email, payment_status, is_private, password_hash, file_path, creator_ip, price, mp_payment_id, mp_qr_code, mp_qr_base64, mp_ticket_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, originalURL, maxViews, expiresAt, tierReq, email, paymentStatus, isPrivate, passwordHash, savedFilePath, ipHash,
		price, mpPaymentID, mpQRCode, mpQRBase64, mpTicketURL)

	if errDB != nil {
		log.Printf("[DB ERR] Falha ao inserir link: %v", errDB)
		http.Error(w, "Failed to create resource", http.StatusInternalServerError)
		return
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	pixelUrl := baseURL + "/i/" + id
	previewUrl := baseURL + "/p/" + id

	if email != "" {
		go func(e, lid, pwd, status, ticketURL string) {
			subject := "Crom-Vision - Ativo Operante!"
			body := fmt.Sprintf(`<h2>Gerenciador de Ativos Crom-Vision</h2>
			<p>Seu ativo <strong>%s</strong> foi processado.</p>
			<p><b>URL do Rastreamento:</b> <a href="%s">%s</a></p>
			<p><b>Acessar Dashboard:</b> <a href="%s/?dashboard=%s">%s/?dashboard=%s</a></p>`, lid, pixelUrl, pixelUrl, baseURL, lid, baseURL, lid)

			if status == "pending" {
				body += fmt.Sprintf(`<hr><p style="color:#d97706"><b>Atenção:</b> Seu sistema aguarda o pagamento. Quando for aprovado, seu link será liberado.</p>
				<p>Sua Senha Mágica para acessar o Relatório Privado é: <b>%s</b></p>`, pwd)
				
				// Incluir link de pagamento do Mercado Pago
				if ticketURL != "" {
					body += fmt.Sprintf(`<hr><h3 style="color:#10b981">💳 Pagar com PIX</h3>
					<p>Clique no link abaixo para pagar via PIX (Mercado Pago):</p>
					<p><a href="%s" style="display:inline-block;background:#10b981;color:white;padding:12px 24px;border-radius:8px;text-decoration:none;font-weight:bold">Pagar R$ %.2f via PIX →</a></p>`, ticketURL, price)
				}
			} else {
				if pwd != "" {
					body += fmt.Sprintf(`<p>Seu tracker está ativo!</p><p>Sua Senha Mágica: <b>%s</b></p>`, pwd)
				} else {
					body += `<p>Sua conta Free está ativa, acesse o link do Dashboard acima para acompanhar!</p>`
				}
			}
			services.SendEmail(e, subject, body)
		}(email, id, clearPassword, paymentStatus, mpTicketURL)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":             id,
		"pixel_url":      pixelUrl,
		"preview_url":    previewUrl,
		"price":          price,
		"payment_status": paymentStatus,
		"is_private":     isPrivate,
		"expires_at":     expiresAt,
		"temp_password":  clearPassword,
	})
}
