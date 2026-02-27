package services

import (
	"os"
	"testing"
)

func TestSendEmail_SkipsWhenNoConfig(t *testing.T) {
	// Garantir que variáveis de SMTP estão vazias
	os.Unsetenv("SMTP_HOST")
	os.Unsetenv("SMTP_USER")
	os.Unsetenv("SMTP_PASS")

	// Não deve dar panic
	SendEmail("test@test.com", "Subject", "<p>Body</p>")
	// Se chegou aqui sem panic, o teste passou
}

func TestSendEmail_SkipsPartialConfig(t *testing.T) {
	os.Setenv("SMTP_HOST", "smtp.gmail.com")
	os.Unsetenv("SMTP_USER")
	os.Unsetenv("SMTP_PASS")
	defer os.Unsetenv("SMTP_HOST")

	// Deve pular sem erro (host existe, mas user/pass não)
	SendEmail("test@test.com", "Subject", "Body")
}

func TestSendEmail_DoesNotPanicWithInvalidConfig(t *testing.T) {
	os.Setenv("SMTP_HOST", "invalid.host.fake")
	os.Setenv("SMTP_PORT", "587")
	os.Setenv("SMTP_USER", "fake@fake.com")
	os.Setenv("SMTP_PASS", "fakepass")
	defer func() {
		os.Unsetenv("SMTP_HOST")
		os.Unsetenv("SMTP_PORT")
		os.Unsetenv("SMTP_USER")
		os.Unsetenv("SMTP_PASS")
	}()

	// Deve logar erro mas não deve dar panic
	SendEmail("dest@test.com", "Test", "Body")
}
