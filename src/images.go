package src

import (
	b64 "encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
)

// Maximum upload size: 10 MB
const maxUploadSize = 10 * 1024 * 1024

// Allowed image file extensions
var allowedImageExtensions = map[string]bool{
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".svg":  true,
	".webp": true,
	".ico":  true,
	".bmp":  true,
}

func uploadLogo(input, filename string) (logoURL string, err error) {

	// Sanitize filename to prevent path traversal attacks
	filename = filepath.Base(filename)
	if filename == "." || filename == ".." || filename == "" {
		err = fmt.Errorf("invalid filename")
		return
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedImageExtensions[ext] {
		err = fmt.Errorf("file type not allowed: %s (allowed: png, jpg, jpeg, gif, svg, webp, ico, bmp)", ext)
		return
	}

	commaIdx := strings.IndexByte(input, ',')
	if commaIdx < 0 || commaIdx >= len(input)-1 {
		err = fmt.Errorf("invalid base64 input format")
		return
	}
	b64data := input[commaIdx+1:]

	// Base64 in bytes umwandeln und speichern
	sDec, err := b64.StdEncoding.DecodeString(b64data)
	if err != nil {
		return
	}

	// Validate file size
	if len(sDec) > maxUploadSize {
		err = fmt.Errorf("file too large: %d bytes (max: %d bytes)", len(sDec), maxUploadSize)
		return
	}

	var file = fmt.Sprintf("%s%s", System.Folder.ImagesUpload, filename)

	err = writeByteToFile(file, sDec)
	if err != nil {
		return
	}

	// Respect Force HTTPS setting when generating logo URL
	if Settings.ForceHttps && Settings.HttpsThreadfinDomain != "" {
		logoURL = fmt.Sprintf("https://%s:%d/data_images/%s", Settings.HttpsThreadfinDomain, Settings.HttpsPort, filename)
	} else if Settings.HttpThreadfinDomain != "" {
		logoURL = fmt.Sprintf("http://%s:%s/data_images/%s", Settings.HttpThreadfinDomain, Settings.Port, filename)
	} else {
		logoURL = fmt.Sprintf("%s://%s/data_images/%s", System.ServerProtocol.XML, System.Domain, filename)
	}

	return

}
