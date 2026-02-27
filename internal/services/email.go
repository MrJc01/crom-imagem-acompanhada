package services

import (
	"log"
	"net/smtp"
	"os"
)

func SendEmail(to, subject, body string) {
	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")

	if host == "" || user == "" || pass == "" {
		log.Println("[EMAIL SKIP] Variáveis ausentes para SMTP. Não enviaremos email para:", to)
		return
	}

	auth := smtp.PlainAuth("", user, pass, host)

	// RFC 822 format headers
	msg := []byte("To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n" +
		body + "\r\n")

	err := smtp.SendMail(host+":"+port, auth, user, []string{to}, msg)
	if err != nil {
		log.Printf("[CRIT] Falha enviando E-mail para %s: %v", to, err)
	} else {
		log.Printf("[MAIL] E-mail enviado com sucesso para %s", to)
	}
}
