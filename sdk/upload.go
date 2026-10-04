package sdk

import (
	"context"
	"fmt"
	"net/url"
)

// GetPresignedUpload asks the Common API for a presigned S3 upload URL.
func (c *Client) GetPresignedUpload(ctx context.Context, req PresignUploadRequest) (*PresignUploadResponse, error) {
	var resp PresignUploadResponse
	err := c.do(ctx, "POST", "/api/v1/upload/presigned-url", req, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// Confirm marks an uploaded file as confirmed in the Common API so S3 lifecycle never deletes it.
// This is the primary 1-line call used in billing-management after saving a form.
func (c *Client) Confirm(ctx context.Context, fileKey string) (*ConfirmUploadResponse, error) {
	if fileKey == "" {
		return nil, fmt.Errorf("fileKey cannot be empty")
	}

	payload := map[string]string{"fileKey": fileKey}
	var resp ConfirmUploadResponse
	err := c.do(ctx, "POST", "/api/v1/upload/confirm", payload, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPresignedDownload requests a temporary signed URL for viewing/downloading private files.
func (c *Client) GetPresignedDownload(ctx context.Context, fileKey string, expiresInMinutes int) (*PresignDownloadResponse, error) {
	payload := map[string]interface{}{
		"fileKey":          fileKey,
		"expiresInMinutes": expiresInMinutes,
	}

	var resp PresignDownloadResponse
	err := c.do(ctx, "POST", "/api/v1/upload/presigned-download-url", payload, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetMetadata fetches file details from S3 via Common API.
func (c *Client) GetMetadata(ctx context.Context, fileKey string) (*FileMetadata, error) {
	escapedKey := url.QueryEscape(fileKey)
	path := fmt.Sprintf("/api/v1/upload/info?key=%s", escapedKey)

	var resp FileMetadata
	err := c.do(ctx, "GET", path, nil, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteFile deletes a single file from S3 via Common API.
func (c *Client) DeleteFile(ctx context.Context, fileKey string) error {
	escapedKey := url.QueryEscape(fileKey)
	path := fmt.Sprintf("/api/v1/upload?key=%s", escapedKey)
	return c.do(ctx, "DELETE", path, nil, nil)
}

// BatchDeleteFiles removes multiple files in a single request.
func (c *Client) BatchDeleteFiles(ctx context.Context, fileKeys []string) error {
	payload := map[string][]string{"fileKeys": fileKeys}
	return c.do(ctx, "POST", "/api/v1/upload/batch-delete", payload, nil)
}
