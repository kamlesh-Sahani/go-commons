package sdk

import "time"

// PresignUploadRequest is sent to the Common API to generate an upload ticket.
type PresignUploadRequest struct {
	FileName         string `json:"fileName"`
	FileType         string `json:"fileType"`
	FileSize         int64  `json:"fileSize,omitempty"`
	Folder           string `json:"folder,omitempty"`
	IsPublic         *bool  `json:"isPublic,omitempty"`
	ExpiresInMinutes int    `json:"expiresInMinutes,omitempty"`
}

// PresignUploadResponse is returned by Common API with the direct S3 upload URL.
type PresignUploadResponse struct {
	UploadURL string            `json:"uploadUrl"`
	FileKey   string            `json:"fileKey"`
	FileURL   string            `json:"fileUrl"`
	ExpiresIn int64             `json:"expiresIn"`
	Headers   map[string]string `json:"headers,omitempty"`
}

// ConfirmUploadResponse is returned after confirming an upload.
type ConfirmUploadResponse struct {
	FileKey     string `json:"fileKey"`
	FileURL     string `json:"fileUrl"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Status      string `json:"status"` // "confirmed"
}

// PresignDownloadResponse returns temporary download URL for private files.
type PresignDownloadResponse struct {
	DownloadURL string `json:"downloadUrl"`
	FileKey     string `json:"fileKey"`
	ExpiresIn   int64  `json:"expiresIn"`
}

// FileMetadata holds metadata details of a file in S3.
type FileMetadata struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"contentType"`
	LastModified time.Time `json:"lastModified"`
	ETag         string    `json:"etag"`
}
