package utils

import "encoding/base64"

var (
	// TransparentGif 1x1 em GIF decodificado a partir de Base64
	TransparentGif, _ = base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7")
)
