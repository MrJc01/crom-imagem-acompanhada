package utils

import (
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
)

var (
	antiF5Cache    = make(map[string]time.Time)
	antiF5Mutex    sync.RWMutex
	CooldownPeriod = 5 * time.Minute

	geoDB   *geoip2.Reader
	geoOnce sync.Once
)

// InitGeoIP carrega o banco MaxMind GeoLite2-City.mmdb localmente.
// Se o arquivo não existir, opera em modo fallback (placeholder).
func InitGeoIP() {
	geoOnce.Do(func() {
		dbPath := os.Getenv("GEOIP_DB_PATH")
		if dbPath == "" {
			dbPath = "./GeoLite2-City.mmdb"
		}

		var err error
		geoDB, err = geoip2.Open(dbPath)
		if err != nil {
			log.Printf("[GEO] ⚠️  GeoLite2 não encontrado em %s — operando em modo placeholder. Erro: %v", dbPath, err)
			log.Println("[GEO] Para GeoIP real, baixe GeoLite2-City.mmdb de https://dev.maxmind.com/geoip/geolite2-free-geolocation-data")
			geoDB = nil
		} else {
			log.Printf("[GEO] ✅ GeoLite2-City carregado com sucesso de %s", dbPath)
		}
	})
}

// CloseGeoIP fecha o banco GeoIP na finalização.
func CloseGeoIP() {
	if geoDB != nil {
		geoDB.Close()
	}
}

// LookupGeoIP resolve a localização real do IP usando banco local.
// Se o banco não estiver carregado, cai no fallback placeholder.
func LookupGeoIP(rawIP string) (country, city string) {
	// Limpar porta do IP se existir (ex: "10.0.0.1:54321" → "10.0.0.1")
	ip := rawIP
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		host, _, err := net.SplitHostPort(ip)
		if err == nil {
			ip = host
		}
	}

	// Se banco GeoIP não carregou, usa fallback
	if geoDB == nil {
		return PlaceholderGeoLocation(ip)
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "??", "Unknown"
	}

	// Lookup local — zero chamadas externas
	record, err := geoDB.City(parsed)
	if err != nil {
		return "??", "Unknown"
	}

	country = record.Country.IsoCode
	if country == "" {
		country = "??"
	}

	// Tenta nome da cidade em pt-BR, depois en, depois o default
	city = record.City.Names["pt-BR"]
	if city == "" {
		city = record.City.Names["en"]
	}
	if city == "" {
		city = "Unknown"
	}

	return country, city
}

// PlaceholderGeoLocation é o fallback quando GeoLite2 não está disponível.
// Retorna localização simulada baseada no último caractere do hash/IP.
func PlaceholderGeoLocation(ipOrHash string) (country, city string) {
	if len(ipOrHash) > 0 {
		char := ipOrHash[len(ipOrHash)-1]
		switch {
		case char >= '0' && char <= '3':
			return "BR", "São Paulo"
		case char >= '4' && char <= '6':
			return "US", "New York"
		case char >= '7' && char <= '9':
			return "PT", "Lisbon"
		case char >= 'a' && char <= 'c':
			return "JP", "Tokyo"
		case char >= 'd' && char <= 'f':
			return "DE", "Berlin"
		}
	}
	return "BR", "Desconhecida"
}

func IsUniqueAccess(fingerprintKey string) bool {
	antiF5Mutex.Lock()
	defer antiF5Mutex.Unlock()
	lastAccess, exists := antiF5Cache[fingerprintKey]
	if exists && time.Since(lastAccess) < CooldownPeriod {
		return false
	}
	antiF5Cache[fingerprintKey] = time.Now()
	return true
}

func CleanupAntiF5() {
	for {
		time.Sleep(15 * time.Minute)
		antiF5Mutex.Lock()
		now := time.Now()
		cleanedCount := 0
		for k, v := range antiF5Cache {
			if now.Sub(v) > CooldownPeriod {
				delete(antiF5Cache, k)
				cleanedCount++
			}
		}
		antiF5Mutex.Unlock()
		if cleanedCount > 0 {
			log.Printf("[🧹] RAM Cleanup: %d hashes expirados limpos", cleanedCount)
		}
	}
}
