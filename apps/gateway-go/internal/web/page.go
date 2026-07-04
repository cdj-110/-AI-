package web

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed frontend/index.html
var indexHTML string

//go:embed frontend/brand-logo.png
var brandLogoPNG []byte

func renderIndexHTML() string {
	return strings.Replace(indexHTML, "__IEC61850_ENABLED__", boolLiteral(iec61850Enabled), 1)
}

func serveBrandLogo(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "image/png")
	writer.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = writer.Write(brandLogoPNG)
}
