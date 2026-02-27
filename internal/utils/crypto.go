package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
)

func GenerateRandomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func ComposeFingerprintHash(ip, ua string) string {
	salt := os.Getenv("APP_SALT")
	if salt == "" {
		salt = "default_salt"
	}
	h := sha256.New()
	h.Write([]byte(ip + "-salt-" + salt + "-" + ua))
	return hex.EncodeToString(h.Sum(nil))
}
