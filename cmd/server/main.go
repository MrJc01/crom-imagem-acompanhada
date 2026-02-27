package main

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"

	"crom-vision/internal/database"
	"crom-vision/internal/handlers"
	"crom-vision/internal/services"
	"crom-vision/internal/utils"
)

// corsMiddleware aplica CORS baseado na variável CORS_ORIGINS do .env
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed := os.Getenv("CORS_ORIGINS")
		if allowed == "" {
			allowed = "*"
		}

		origin := r.Header.Get("Origin")
		if allowed == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			for _, o := range strings.Split(allowed, ",") {
				if strings.TrimSpace(o) == origin {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// originGuard rejeita requisições POST/PUT/PATCH de origens não autorizadas.
// Usa a variável ALLOWED_FORM_ORIGINS do .env (separada por vírgula).
// Se ALLOWED_FORM_ORIGINS="*" ou vazia, aceita qualquer origem.
func originGuard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Só valida métodos que modificam estado
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		allowed := os.Getenv("ALLOWED_FORM_ORIGINS")
		if allowed == "" || allowed == "*" {
			next.ServeHTTP(w, r)
			return
		}

		// Extrai a origem do header Origin ou, na ausência, do Referer
		incoming := r.Header.Get("Origin")
		if incoming == "" {
			if ref := r.Header.Get("Referer"); ref != "" {
				if parsed, err := url.Parse(ref); err == nil {
					incoming = parsed.Scheme + "://" + parsed.Host
				}
			}
		}

		if incoming == "" {
			log.Printf("[GUARD] ❌ Requisição %s %s bloqueada — sem header Origin/Referer", r.Method, r.URL.Path)
			http.Error(w, "Origem não identificada. Requisição bloqueada.", http.StatusForbidden)
			return
		}

		for _, o := range strings.Split(allowed, ",") {
			if strings.TrimSpace(o) == incoming {
				next.ServeHTTP(w, r)
				return
			}
		}

		log.Printf("[GUARD] ❌ Origem '%s' não permitida para %s %s", incoming, r.Method, r.URL.Path)
		http.Error(w, "Origem não autorizada. Configure ALLOWED_FORM_ORIGINS no servidor.", http.StatusForbidden)
	}
}

func main() {
	// Carrega variáveis do .env (se existir)
	if err := godotenv.Load(); err != nil {
		log.Println("[ENV] Arquivo .env não encontrado, usando variáveis de ambiente do sistema")
	} else {
		log.Println("[ENV] ✅ Variáveis carregadas do .env")
	}

	database.InitDB()
	defer database.DB.Close()

	utils.InitGeoIP()
	defer utils.CloseGeoIP()

	go utils.CleanupAntiF5()
	go services.HardDeleteExpired()

	mux := http.NewServeMux()

	// Rotas de negócio
	mux.HandleFunc("/i/", handlers.ImageHandler)
	mux.HandleFunc("/api/checkout", originGuard(handlers.CheckoutHandler))
	mux.HandleFunc("/api/webhook/mp", handlers.WebhookMPHandler) // Webhook MP não usa originGuard (vem do Mercado Pago)
	mux.HandleFunc("/api/public-links", handlers.PublicLinksHandler)
	mux.HandleFunc("/api/private-stats", handlers.PrivateStatsHandler)
	mux.HandleFunc("/api/link-stats", handlers.LinkStatsHandler)
	mux.HandleFunc("/api/link-geo", handlers.LinkGeoHandler)
	mux.HandleFunc("/p/", handlers.PreviewHandler)

	// LGPD
	mux.HandleFunc("/api/lgpd/consultar", originGuard(handlers.LGPDConsultarHandler))
	mux.HandleFunc("/api/lgpd/apagar", originGuard(handlers.LGPDApagarHandler))

	// Health check
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Config
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		mode := os.Getenv("APP_MODE")
		if mode == "" {
			mode = "saas"
		}
		geoEnabled := "true"
		if strings.ToLower(os.Getenv("GEO_TRACKING_ENABLED")) == "false" {
			geoEnabled = "false"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"app_mode":             mode,
			"geo_tracking_enabled": geoEnabled,
		})
	})

	// Arquivos estáticos
	mux.Handle("/", http.FileServer(http.Dir("./public")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 Imagem Acompanhada v5 operando na porta :%s\n", port)
	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		log.Fatalf("[FALHA] Servidor HTTP interrompido: %v", err)
	}
}
