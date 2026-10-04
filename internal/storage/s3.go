package storage

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	s3InitMu       sync.RWMutex
	S3Client       *s3.Client
	PresignClient  *s3.PresignClient
	AwsS3Bucket    string
	AwsRegion      string
	CdnBaseURL     string
	CustomEndpoint string
	ForcePathStyle bool
)

// EnsureS3Initialized provides thread-safe initialization of S3 clients.
func EnsureS3Initialized() error {
	s3InitMu.RLock()
	if S3Client != nil && PresignClient != nil {
		s3InitMu.RUnlock()
		return nil
	}
	s3InitMu.RUnlock()

	s3InitMu.Lock()
	defer s3InitMu.Unlock()

	if S3Client != nil && PresignClient != nil {
		return nil
	}
	return InitS3()
}

// ObjectMetadata holds metadata details of an S3 object.
type ObjectMetadata struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"contentType"`
	LastModified time.Time `json:"lastModified"`
	ETag         string    `json:"etag"`
	Status       string    `json:"status"`
}

// InitS3 loads AWS configuration and sets up the S3 client.
func InitS3() error {
	AwsS3Bucket = os.Getenv("AWS_S3_BUCKET")
	if AwsS3Bucket == "" {
		return fmt.Errorf("AWS_S3_BUCKET environment variable is strictly required")
	}

	AwsRegion = os.Getenv("AWS_REGION")
	if AwsRegion == "" {
		AwsRegion = "us-east-1"
	}

	CdnBaseURL = strings.TrimRight(os.Getenv("PUBLIC_CDN_BASE_URL"), "/")
	CustomEndpoint = os.Getenv("AWS_ENDPOINT")
	ForcePathStyle = os.Getenv("AWS_S3_FORCE_PATH_STYLE") == "true"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var optFns []func(*config.LoadOptions) error
	optFns = append(optFns, config.WithRegion(AwsRegion))

	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if accessKey != "" && secretKey != "" {
		optFns = append(optFns, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	}

	cfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return fmt.Errorf("unable to load AWS configuration: %w", err)
	}

	S3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		if CustomEndpoint != "" {
			o.BaseEndpoint = aws.String(CustomEndpoint)
		}
		o.UsePathStyle = ForcePathStyle
	})

	PresignClient = s3.NewPresignClient(S3Client)
	return nil
}

// GeneratePresignedUpload creates a signed PUT URL initially tagged status=pending.
func GeneratePresignedUpload(ctx context.Context, key, contentType string, expiresIn time.Duration) (string, error) {
	if err := EnsureS3Initialized(); err != nil {
		return "", err
	}

	if expiresIn <= 0 {
		expiresIn = 15 * time.Minute
	}

	req, err := PresignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(AwsS3Bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
		Tagging:     aws.String("status=pending"),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})
	if err != nil {
		return "", fmt.Errorf("could not generate presigned PUT URL: %w", err)
	}

	return req.URL, nil
}

// ConfirmObject verifies file exists and updates its tag to status=confirmed.
func ConfirmObject(ctx context.Context, key string) (*ObjectMetadata, error) {
	if err := EnsureS3Initialized(); err != nil {
		return nil, err
	}

	head, err := S3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(AwsS3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("file not found in S3 or upload was incomplete: %w", err)
	}

	// Update tag to status=confirmed
	_, err = S3Client.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
		Bucket: aws.String(AwsS3Bucket),
		Key:    aws.String(key),
		Tagging: &types.Tagging{
			TagSet: []types.Tag{
				{
					Key:   aws.String("status"),
					Value: aws.String("confirmed"),
				},
				{
					Key:   aws.String("confirmed_at"),
					Value: aws.String(time.Now().UTC().Format(time.RFC3339)),
				},
			},
		},
	})
	if err != nil {
		fmt.Printf("[Warning] Failed to update S3 object tagging for %s: %v\n", key, err)
	}

	var size int64
	if head.ContentLength != nil {
		size = *head.ContentLength
	}

	var contentType string
	if head.ContentType != nil {
		contentType = *head.ContentType
	}

	var etag string
	if head.ETag != nil {
		etag = strings.Trim(*head.ETag, "\"")
	}

	var lastModified time.Time
	if head.LastModified != nil {
		lastModified = *head.LastModified
	}

	return &ObjectMetadata{
		Key:          key,
		Size:         size,
		ContentType:  contentType,
		LastModified: lastModified,
		ETag:         etag,
		Status:       "confirmed",
	}, nil
}

// GeneratePresignedDownload creates a temporary signed GET URL.
func GeneratePresignedDownload(ctx context.Context, key string, expiresIn time.Duration) (string, error) {
	if err := EnsureS3Initialized(); err != nil {
		return "", err
	}

	if expiresIn <= 0 {
		expiresIn = 15 * time.Minute
	}

	req, err := PresignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(AwsS3Bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})
	if err != nil {
		return "", fmt.Errorf("could not generate presigned download URL: %w", err)
	}

	return req.URL, nil
}

// GetObjectMetadata retrieves object details from S3.
func GetObjectMetadata(ctx context.Context, key string) (*ObjectMetadata, error) {
	if err := EnsureS3Initialized(); err != nil {
		return nil, err
	}

	head, err := S3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(AwsS3Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("could not get object metadata: %w", err)
	}

	var size int64
	if head.ContentLength != nil {
		size = *head.ContentLength
	}

	var contentType string
	if head.ContentType != nil {
		contentType = *head.ContentType
	}

	var etag string
	if head.ETag != nil {
		etag = strings.Trim(*head.ETag, "\"")
	}

	var lastModified time.Time
	if head.LastModified != nil {
		lastModified = *head.LastModified
	}

	return &ObjectMetadata{
		Key:          key,
		Size:         size,
		ContentType:  contentType,
		LastModified: lastModified,
		ETag:         etag,
		Status:       "exists",
	}, nil
}

// DeleteS3Object removes an object from S3.
func DeleteS3Object(ctx context.Context, key string) error {
	if err := EnsureS3Initialized(); err != nil {
		return err
	}

	_, err := S3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(AwsS3Bucket),
		Key:    aws.String(key),
	})
	return err
}

// DeleteS3Objects deletes multiple objects in batches of at most 1000 items (AWS limit).
func DeleteS3Objects(ctx context.Context, keys []string) error {
	if err := EnsureS3Initialized(); err != nil {
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	const maxBatchSize = 1000
	for i := 0; i < len(keys); i += maxBatchSize {
		end := i + maxBatchSize
		if end > len(keys) {
			end = len(keys)
		}
		chunk := keys[i:end]

		objects := make([]types.ObjectIdentifier, 0, len(chunk))
		for _, key := range chunk {
			objects = append(objects, types.ObjectIdentifier{
				Key: aws.String(key),
			})
		}

		out, err := S3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(AwsS3Bucket),
			Delete: &types.Delete{
				Objects: objects,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return err
		}
		if len(out.Errors) > 0 {
			var errMsgs []string
			for _, e := range out.Errors {
				errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", aws.ToString(e.Key), aws.ToString(e.Message)))
			}
			return fmt.Errorf("failed to delete some S3 objects: %s", strings.Join(errMsgs, "; "))
		}
	}

	return nil
}

// BuildPublicURL constructs permanent URL for public files.
func BuildPublicURL(key string) string {
	if CdnBaseURL != "" {
		return fmt.Sprintf("%s/%s", CdnBaseURL, key)
	}

	if CustomEndpoint != "" {
		if ForcePathStyle {
			return fmt.Sprintf("%s/%s/%s", strings.TrimRight(CustomEndpoint, "/"), AwsS3Bucket, key)
		}
		return fmt.Sprintf("%s/%s", strings.TrimRight(CustomEndpoint, "/"), key)
	}

	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", AwsS3Bucket, AwsRegion, key)
}
