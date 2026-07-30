package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed frontend/packet-monitor.html
var packetMonitorHTML string

//go:embed frontend/brand-logo.png
var brandLogoPNG []byte

//go:embed frontend/vue-dist
var vueFrontendFiles embed.FS

func renderPacketMonitorHTML() string { return packetMonitorHTML }

func vueFrontendHandler() http.Handler {
	root, err := fs.Sub(vueFrontendFiles, "frontend/vue-dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/" || request.URL.Path == "/index.html" {
			writer.Header().Set("Cache-Control", "no-store")
		} else if len(request.URL.Path) > len("/assets/") && request.URL.Path[:len("/assets/")] == "/assets/" {
			writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(writer, request)
	})
}

func serveBrandLogo(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "image/png")
	writer.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = writer.Write(brandLogoPNG)
}
