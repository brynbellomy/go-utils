package bhttp

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

type sniffedReadCloser struct {
	io.Reader
	io.Closer
}

func SniffContentType(filename string, data io.ReadCloser) (io.ReadCloser, string, error) {
	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)

	n, err := data.Read(buffer)
	if err != nil {
		return nil, "", err
	}
	buffer = buffer[:n]

	// Use the net/http package's handy DectectContentType function. Always returns a valid
	// content-type by returning "application/octet-stream" if no others seemed to match.
	contentType := http.DetectContentType(buffer)

	// If we got an ambiguous result, check the file extension
	if contentType == "application/octet-stream" {
		contentType = GuessContentTypeFromFilename(filename)
	}

	newReadCloser := sniffedReadCloser{
		Reader: io.MultiReader(bytes.NewReader(buffer), data),
		Closer: data,
	}
	return newReadCloser, contentType, nil
}

func GuessContentTypeFromFilename(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) > 1 {
		ext := strings.ToLower(parts[len(parts)-1])
		switch ext {
		case "txt":
			return "text/plain"
		case "html":
			return "text/html"
		case "js":
			return "application/js"
		case "json":
			return "application/json"
		case "png":
			return "image/png"
		case "jpg", "jpeg":
			return "image/jpeg"
		}
	}
	return "application/octet-stream"
}
