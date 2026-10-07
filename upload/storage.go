package upload

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Driver abstracts the underlying storage mechanism (S3, MinIO, Cloudflare R2, Local Disk).
type Driver interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, keys []string) error
	PresignPut(ctx context.Context, key, contentType string, expiresIn time.Duration) (string, error)
	PublicURL(key string) string
}

var (
	driverMu      sync.RWMutex
	defaultDriver Driver
)

// SetDriver overrides the default storage driver (useful for tests or custom backends).
func SetDriver(d Driver) {
	driverMu.Lock()
	defer driverMu.Unlock()
	defaultDriver = d
}

// GetDriver returns the initialized Driver or auto-configures one from environment variables.
func GetDriver() (Driver, error) {
	driverMu.RLock()
	if defaultDriver != nil {
		defer driverMu.RUnlock()
		return defaultDriver, nil
	}
	driverMu.RUnlock()

	driverMu.Lock()
	defer driverMu.Unlock()

	if defaultDriver != nil {
		return defaultDriver, nil
	}

	driver, err := initDriverFromEnv()
	if err != nil {
		return nil, err
	}
	defaultDriver = driver
	return defaultDriver, nil
}

func initDriverFromEnv() (Driver, error) {
	driverType := strings.ToLower(strings.TrimSpace(os.Getenv("UPLOAD_STORAGE_DRIVER")))

	bucket := os.Getenv("AWS_S3_BUCKET")
	if bucket == "" {
		bucket = os.Getenv("S3_BUCKET")
	}

	// Default to local driver if explicitly configured or if no S3 bucket is set in dev mode
	if driverType == "local" || (bucket == "" && os.Getenv("ENV") != "production") {
		localPath := os.Getenv("UPLOAD_LOCAL_PATH")
		if localPath == "" {
			localPath = "./uploads"
		}
		return NewLocalDriver(localPath), nil
	}

	if bucket == "" {
		return nil, fmt.Errorf("upload: AWS_S3_BUCKET environment variable is required")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	endpoint := os.Getenv("AWS_ENDPOINT")
	forcePathStyle := os.Getenv("AWS_S3_FORCE_PATH_STYLE") == "true"
	cdnBaseURL := os.Getenv("PUBLIC_CDN_BASE_URL")

	return NewS3Driver(S3Config{
		Bucket:         bucket,
		Region:         region,
		AccessKeyID:    accessKey,
		SecretKey:      secretKey,
		Endpoint:       endpoint,
		ForcePathStyle: forcePathStyle,
		CDNBaseURL:     cdnBaseURL,
	})
}

// --- S3 Driver ---

type S3Config struct {
	Bucket         string
	Region         string
	AccessKeyID    string
	SecretKey      string
	Endpoint       string
	ForcePathStyle bool
	CDNBaseURL     string
}

type S3Driver struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	region        string
	cdnBaseURL    string
	endpoint      string
}

func NewS3Driver(cfg S3Config) (*S3Driver, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var optFns []func(*config.LoadOptions) error
	if cfg.Region != "" {
		optFns = append(optFns, config.WithRegion(cfg.Region))
	}

	if cfg.AccessKeyID != "" && cfg.SecretKey != "" {
		optFns = append(optFns, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("upload: failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	})

	presignClient := s3.NewPresignClient(s3Client)

	return &S3Driver{
		client:        s3Client,
		presignClient: presignClient,
		bucket:        cfg.Bucket,
		region:        cfg.Region,
		cdnBaseURL:    strings.TrimRight(cfg.CDNBaseURL, "/"),
		endpoint:      cfg.Endpoint,
	}, nil
}

func (s *S3Driver) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("upload: s3 put object error: %w", err)
	}
	return nil
}

func (s *S3Driver) Delete(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	var objects []types.ObjectIdentifier
	for _, k := range keys {
		clean := strings.TrimSpace(k)
		if clean != "" {
			objects = append(objects, types.ObjectIdentifier{Key: aws.String(clean)})
		}
	}
	if len(objects) == 0 {
		return nil
	}

	_, err := s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(s.bucket),
		Delete: &types.Delete{
			Objects: objects,
			Quiet:   aws.Bool(true),
		},
	})
	if err != nil {
		return fmt.Errorf("upload: s3 delete objects error: %w", err)
	}
	return nil
}

func (s *S3Driver) PresignPut(ctx context.Context, key, contentType string, expiresIn time.Duration) (string, error) {
	if expiresIn <= 0 {
		expiresIn = 15 * time.Minute
	}

	req, err := s.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})
	if err != nil {
		return "", fmt.Errorf("upload: failed to presign put object: %w", err)
	}
	return req.URL, nil
}

func (s *S3Driver) PublicURL(key string) string {
	cleanKey := strings.TrimLeft(key, "/")
	if s.cdnBaseURL != "" {
		return fmt.Sprintf("%s/%s", s.cdnBaseURL, cleanKey)
	}
	if s.endpoint != "" {
		return fmt.Sprintf("%s/%s/%s", strings.TrimRight(s.endpoint, "/"), s.bucket, cleanKey)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, cleanKey)
}

// --- Local Driver (for offline / dev / tests) ---

type LocalDriver struct {
	baseDir string
	baseURL string
}

func NewLocalDriver(baseDir string) *LocalDriver {
	_ = os.MkdirAll(baseDir, 0755)
	return &LocalDriver{
		baseDir: baseDir,
		baseURL: "/uploads",
	}
}

func (l *LocalDriver) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	fullPath := filepath.Join(l.baseDir, filepath.FromSlash(key))
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("upload: failed to create local directory: %w", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("upload: failed to create local file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, body); err != nil {
		return fmt.Errorf("upload: failed to write local file: %w", err)
	}
	return nil
}

func (l *LocalDriver) Delete(ctx context.Context, keys []string) error {
	for _, k := range keys {
		clean := filepath.Clean(k)
		if strings.Contains(clean, "..") {
			continue
		}
		fullPath := filepath.Join(l.baseDir, filepath.FromSlash(clean))
		_ = os.Remove(fullPath)
	}
	return nil
}

func (l *LocalDriver) PresignPut(ctx context.Context, key, contentType string, expiresIn time.Duration) (string, error) {
	// For local dev, return the direct upload URL path
	return fmt.Sprintf("%s/%s", l.baseURL, key), nil
}

func (l *LocalDriver) PublicURL(key string) string {
	return fmt.Sprintf("%s/%s", l.baseURL, strings.TrimLeft(key, "/"))
}
