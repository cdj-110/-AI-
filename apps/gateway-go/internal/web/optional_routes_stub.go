//go:build !iec61850_mms || !cgo

package web

import "net/http"

func (s *Server) registerOptionalRoutes(_ *http.ServeMux) {}
