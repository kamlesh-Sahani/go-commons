package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSDKClient(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "unauthorized",
			})
			return
		}

		switch r.URL.Path {
		case "/api/v1/upload/presigned-url":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "success",
				"data": PresignUploadResponse{
					UploadURL: "https://s3.amazonaws.com/test",
					FileKey:   "projects/billpro/invoices/test.pdf",
					FileURL:   "https://cdn.test.com/test.pdf",
					ExpiresIn: 900,
				},
			})
		case "/api/v1/upload/confirm":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"message": "confirmed",
				"data": ConfirmUploadResponse{
					FileKey: "projects/billpro/invoices/test.pdf",
					Status:  "confirmed",
					Size:    1024,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	client := NewClient(mockServer.URL, "test-api-key")

	t.Run("GetPresignedUpload succeeds", func(t *testing.T) {
		res, err := client.GetPresignedUpload(context.Background(), PresignUploadRequest{
			FileName: "test.pdf",
			FileType: "application/pdf",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.FileKey != "projects/billpro/invoices/test.pdf" {
			t.Errorf("unexpected file key: %s", res.FileKey)
		}
	})

	t.Run("Confirm succeeds", func(t *testing.T) {
		res, err := client.Confirm(context.Background(), "projects/billpro/invoices/test.pdf")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "confirmed" {
			t.Errorf("expected confirmed status, got %s", res.Status)
		}
	})

	t.Run("Unauthorized key returns error", func(t *testing.T) {
		badClient := NewClient(mockServer.URL, "wrong-key")
		_, err := badClient.Confirm(context.Background(), "some-key")
		if err == nil {
			t.Errorf("expected unauthorized error, got nil")
		}
	})
}
