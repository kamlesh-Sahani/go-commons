package upload

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gin-gonic/gin"
)

var (
	s3InitOnce sync.Once
	s3Client   *s3.Client
	s3Bucket   string
)

// Save reads uploaded files from the request, validates them, saves them, and returns an array of File.
func Save(c *gin.Context, opt ...Options) ([]*File, error) {
	if c == nil || c.Request == nil {
		return nil, fmt.Errorf("upload: invalid gin request")
	}

	opts := Options{Folder: "uploads", FieldName: "file", MaxSizeMB: 50}
	if len(opt) > 0 {
		if opt[0].Folder != "" {
			opts.Folder = opt[0].Folder
		}
		if opt[0].FieldName != "" {
			opts.FieldName = opt[0].FieldName
		}
		if opt[0].MaxSizeMB > 0 {
			opts.MaxSizeMB = opt[0].MaxSizeMB
		}
		opts.AllowedTypes = opt[0].AllowedTypes
		opts.TenantID = opt[0].TenantID
	}

	_ = c.Request.ParseMultipartForm(32 * 1024 * 1024)
	headers := getFiles(c, opts.FieldName)
	if len(headers) == 0 {
		return nil, fmt.Errorf("upload: no file uploaded under field '%s'", opts.FieldName)
	}

	maxBytes := opts.MaxSizeMB * 1024 * 1024
	var result []*File

	for _, fh := range headers {
		if fh.Size > maxBytes {
			return nil, fmt.Errorf("upload: file '%s' exceeds limit of %d MB", fh.Filename, opts.MaxSizeMB)
		}

		cleanName := cleanFileName(fh.Filename)
		ext := strings.ToLower(filepath.Ext(cleanName))

		// Block dangerous script/executable extensions
		if isBlocked(ext) {
			return nil, fmt.Errorf("upload: file '%s' is not allowed for security reasons", fh.Filename)
		}

		// Check allowed types if specified
		if len(opts.AllowedTypes) > 0 && !isAllowed(cleanName, fh.Header.Get("Content-Type"), opts.AllowedTypes) {
			return nil, fmt.Errorf("upload: file type of '%s' is not allowed", fh.Filename)
		}

		key := buildKey(opts.TenantID, opts.Folder, cleanName)
		fileURL, err := storeFile(c.Request.Context(), fh, key)
		if err != nil {
			return nil, err
		}

		result = append(result, &File{
			Name: cleanName,
			URL:  fileURL,
			Key:  key,
			Size: fh.Size,
		})
	}

	return result, nil
}

// Delete removes an array of file keys from storage.
func Delete(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	client, bucket := getS3()
	if client == nil || bucket == "" {
		// Local storage deletion fallback
		localDir := getLocalDir()
		for _, k := range keys {
			_ = os.Remove(filepath.Join(localDir, filepath.FromSlash(k)))
		}
		return nil
	}

	var objs []types.ObjectIdentifier
	for _, k := range keys {
		if clean := strings.TrimSpace(k); clean != "" {
			objs = append(objs, types.ObjectIdentifier{Key: aws.String(clean)})
		}
	}

	_, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{Objects: objs, Quiet: aws.Bool(true)},
	})
	return err
}

// --- Internal Helpers ---

func storeFile(ctx context.Context, fh *multipart.FileHeader, key string) (string, error) {
	src, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("upload: cannot open file: %w", err)
	}
	defer src.Close()

	client, bucket := getS3()
	if client == nil || bucket == "" {
		// Save locally if no S3 bucket configured (great for local tests & dev)
		localDir := getLocalDir()
		destPath := filepath.Join(localDir, filepath.FromSlash(key))
		_ = os.MkdirAll(filepath.Dir(destPath), 0755)

		dst, err := os.Create(destPath)
		if err != nil {
			return "", fmt.Errorf("upload: local write error: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return "", err
		}
		return "/uploads/" + key, nil
	}

	// Upload to S3
	cType := fh.Header.Get("Content-Type")
	if cType == "" {
		cType = "application/octet-stream"
	}

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          src,
		ContentLength: aws.Int64(fh.Size),
		ContentType:   aws.String(cType),
	})
	if err != nil {
		return "", fmt.Errorf("upload: s3 upload error: %w", err)
	}

	if cdn := os.Getenv("PUBLIC_CDN_BASE_URL"); cdn != "" {
		return strings.TrimRight(cdn, "/") + "/" + key, nil
	}
	if ep := os.Getenv("AWS_ENDPOINT"); ep != "" {
		return fmt.Sprintf("%s/%s/%s", strings.TrimRight(ep, "/"), bucket, key), nil
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, region, key), nil
}

func getS3() (*s3.Client, string) {
	s3InitOnce.Do(func() {
		s3Bucket = os.Getenv("AWS_S3_BUCKET")
		if s3Bucket == "" {
			s3Bucket = os.Getenv("S3_BUCKET")
		}
		if s3Bucket == "" || os.Getenv("UPLOAD_STORAGE_DRIVER") == "local" {
			return
		}

		region := os.Getenv("AWS_REGION")
		if region == "" {
			region = "us-east-1"
		}

		var opts []func(*config.LoadOptions) error
		opts = append(opts, config.WithRegion(region))

		accKey := os.Getenv("AWS_ACCESS_KEY_ID")
		secKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
		if accKey != "" && secKey != "" {
			opts = append(opts, config.WithCredentialsProvider(
				credentials.NewStaticCredentialsProvider(accKey, secKey, ""),
			))
		}

		cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
		if err != nil {
			return
		}

		s3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
			if ep := os.Getenv("AWS_ENDPOINT"); ep != "" {
				o.BaseEndpoint = aws.String(ep)
			}
			o.UsePathStyle = os.Getenv("AWS_S3_FORCE_PATH_STYLE") == "true"
		})
	})

	return s3Client, s3Bucket
}

func getFiles(c *gin.Context, field string) []*multipart.FileHeader {
	if c.Request.MultipartForm == nil {
		return nil
	}
	if files, ok := c.Request.MultipartForm.File[field]; ok && len(files) > 0 {
		return files
	}
	if files, ok := c.Request.MultipartForm.File["files"]; ok && len(files) > 0 {
		return files
	}
	for _, files := range c.Request.MultipartForm.File {
		return files
	}
	return nil
}

func getLocalDir() string {
	dir := os.Getenv("UPLOAD_LOCAL_PATH")
	if dir == "" {
		dir = "./uploads"
	}
	return dir
}
