//go:build iec61850_mms && cgo

package web

import "net/http"

func (s *Server) registerOptionalRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/iec61850/browse", s.requirePermission("collection.run", s.browseIEC61850))
}
