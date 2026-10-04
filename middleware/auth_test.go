package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAuthMiddleware(t *testing.T) {
	os.Setenv("AUTH_ENABLED", "true")
	os.Setenv("API_KEYS", "test-key-1:billpro,test-key-2:storefront")

	router := gin.New()
	router.Use(APIKeyAuth())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"projectId": GetProjectID(c)})
	})

	t.Run("Missing key returns 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		router.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", w.Code)
		}
	})

	t.Run("Valid key sets project ID", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-API-Key", "test-key-1")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("Tenant spoofing via X-Project-ID header is rejected", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-API-Key", "test-key-1")
		req.Header.Set("X-Project-ID", "evil-tenant-spoof")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), `"projectId":"billpro"`) {
			t.Fatalf("expected project ID to remain 'billpro', got body: %s", w.Body.String())
		}
	})

	t.Run("Bearer token authentication works", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Authorization", "Bearer test-key-2")
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if !strings.Contains(w.Body.String(), `"projectId":"storefront"`) {
			t.Fatalf("expected project ID 'storefront', got: %s", w.Body.String())
		}
	})
}

func TestCORSMiddleware(t *testing.T) {
	os.Setenv("FRONTEND_URL", "https://app.accurex.com")
	router := gin.New()
	router.Use(CORS())
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	t.Run("Matching origin allows credentials", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("Origin", "https://app.accurex.com")
		router.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "https://app.accurex.com" {
			t.Errorf("unexpected origin header: %s", w.Header().Get("Access-Control-Allow-Origin"))
		}
		if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Errorf("expected credentials to be allowed for matched origin")
		}
	})

	t.Run("Options preflight returns 204", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodOptions, "/ping", nil)
		req.Header.Set("Origin", "https://app.accurex.com")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204 No Content for preflight, got %d", w.Code)
		}
	})
}

func TestRecoveryMiddleware(t *testing.T) {
	router := gin.New()
	router.Use(Recovery())
	router.GET("/panic", func(c *gin.Context) {
		panic("critical database failure simulation")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/panic", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 status on panic, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"success":false`) {
		t.Fatalf("expected success:false JSON envelope, got: %s", w.Body.String())
	}
}
