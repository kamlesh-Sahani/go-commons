package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestResponseHelpers(t *testing.T) {
	router := gin.New()

	router.GET("/success", func(c *gin.Context) {
		Success(c, "success message", gin.H{"id": 123})
	})
	router.GET("/bad-request", func(c *gin.Context) {
		BadRequest(c, "bad request error")
	})

	t.Run("Success helper returns 200 with standard envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/success", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var res APIResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if !res.Success || res.Message != "success message" {
			t.Errorf("unexpected response body: %+v", res)
		}
	})

	t.Run("BadRequest helper returns 400 with success=false", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/bad-request", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}

		var res APIResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Success || res.Message != "bad request error" {
			t.Errorf("unexpected response body: %+v", res)
		}
	})
}
