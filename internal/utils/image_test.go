package utils

import (
	"testing"
)

func TestTransparentGif(t *testing.T) {
	t.Run("não deve ser nil", func(t *testing.T) {
		if TransparentGif == nil {
			t.Fatal("TransparentGif é nil")
		}
	})

	t.Run("deve ter tamanho válido de GIF", func(t *testing.T) {
		if len(TransparentGif) < 10 {
			t.Errorf("tamanho do GIF muito pequeno: %d bytes", len(TransparentGif))
		}
	})

	t.Run("deve começar com magic bytes GIF89a", func(t *testing.T) {
		header := string(TransparentGif[:6])
		if header != "GIF89a" {
			t.Errorf("header inválido: esperava 'GIF89a', obteve %q", header)
		}
	})

	t.Run("deve ser GIF 1x1 pixel", func(t *testing.T) {
		// GIF width está nos bytes 6-7 (little endian), height nos 8-9
		if len(TransparentGif) < 10 {
			t.Fatal("GIF muito curto para verificar dimensões")
		}
		width := int(TransparentGif[6]) | int(TransparentGif[7])<<8
		height := int(TransparentGif[8]) | int(TransparentGif[9])<<8
		if width != 1 || height != 1 {
			t.Errorf("esperava 1x1, obteve %dx%d", width, height)
		}
	})
}
