package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGatewayFaviconUsesBrandLogo(t *testing.T) {
	response := httptest.NewRecorder()
	serveBrandLogo(response, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("favicon response = %d %q", response.Code, response.Header().Get("Content-Type"))
	}
	if len(response.Body.Bytes()) != len(brandLogoPNG) || len(brandLogoPNG) == 0 {
		t.Fatalf("favicon size = %d, brand logo size = %d", response.Body.Len(), len(brandLogoPNG))
	}

	page := httptest.NewRecorder()
	vueFrontendHandler().ServeHTTP(page, httptest.NewRequest(http.MethodGet, "/", nil))
	body, err := io.ReadAll(page.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	for name, html := range map[string]string{
		"gateway": string(body),
		"login": loginHTML,
		"change-password": changePasswordHTML,
		"mfa": mfaLoginHTML,
	} {
		if !strings.Contains(html, `rel="icon"`) || !strings.Contains(html, "/brand-logo.png?v=20260922") {
			t.Fatalf("%s page does not declare the gateway favicon", name)
		}
	}
}
