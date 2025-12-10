package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Uploader stores binary content in an S3-compatible bucket and returns a public URL.
type Uploader interface {
	Upload(ctx context.Context, key string, reader io.Reader, contentType string) (publicURL string, err error)
}

// Client wraps a MinIO/S3 client.
type Client struct {
	bucket         string
	publicBaseURL  string
	client         *minio.Client
	logger         *slog.Logger
	bucketInitOnce sync.Once
	bucketInitErr  error
}

// NewClient configures an uploader using the provided endpoint and credentials.
func NewClient(endpoint string, useSSL bool, accessKey, secretKey, bucket, publicBaseURL string, logger *slog.Logger) (*Client, error) {
	cleanEndpoint := strings.TrimSpace(endpoint)
	if cleanEndpoint == "" {
		return nil, errors.New("s3: endpoint is required")
	}
	if bucket = strings.TrimSpace(bucket); bucket == "" {
		return nil, errors.New("s3: bucket is required")
	}

	clientEndpoint := parseEndpoint(cleanEndpoint)
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(strings.TrimSpace(accessKey), strings.TrimSpace(secretKey), ""),
		Secure: useSSL,
	}
	minioClient, err := minio.New(clientEndpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("s3: create client: %w", err)
	}

	base := strings.TrimSpace(publicBaseURL)
	if base == "" {
		base = cleanEndpoint
	}

	return &Client{
		bucket:        bucket,
		publicBaseURL: strings.TrimRight(base, "/"),
		client:        minioClient,
		logger:        logger,
	}, nil
}

// Upload stores the content and returns a direct URL (bucket is made publicly readable for local demo).
func (c *Client) Upload(ctx context.Context, key string, reader io.Reader, contentType string) (string, error) {
	if reader == nil {
		return "", errors.New("s3: reader is required")
	}
	key = strings.Trim(strings.TrimSpace(key), "/")
	if key == "" {
		return "", errors.New("s3: object key is required")
	}
	if err := c.ensureBucket(ctx); err != nil {
		return "", err
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := c.client.PutObject(ctx, c.bucket, key, reader, -1, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("s3: put object: %w", err)
	}

	publicURL := c.objectURL(key)
	if c.logger != nil {
		c.logger.Info("s3 upload completed", "bucket", c.bucket, "key", key, "url", publicURL)
	}
	return publicURL, nil
}

// NoopUploader fails fast when S3 is unavailable.
type NoopUploader struct{}

func (NoopUploader) Upload(_ context.Context, _ string, _ io.Reader, _ string) (string, error) {
	return "", errors.New("s3 uploader is not configured")
}

func (c *Client) ensureBucket(ctx context.Context) error {
	c.bucketInitOnce.Do(func() {
		exists, err := c.client.BucketExists(ctx, c.bucket)
		if err != nil {
			c.bucketInitErr = fmt.Errorf("s3: check bucket: %w", err)
			return
		}
		if exists {
			return
		}
		if err := c.client.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
			c.bucketInitErr = fmt.Errorf("s3: create bucket: %w", err)
			return
		}
		if err := c.allowPublicRead(ctx); err != nil {
			c.bucketInitErr = err
		}
	})
	return c.bucketInitErr
}

func (c *Client) allowPublicRead(ctx context.Context) error {
	policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, c.bucket)
	if err := c.client.SetBucketPolicy(ctx, c.bucket, policy); err != nil {
		return fmt.Errorf("s3: set bucket policy: %w", err)
	}
	return nil
}

func (c *Client) objectURL(key string) string {
	base := strings.TrimRight(c.publicBaseURL, "/")
	return fmt.Sprintf("%s/%s/%s", base, c.bucket, strings.TrimLeft(key, "/"))
}

func parseEndpoint(endpoint string) string {
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return endpoint
}

var _ Uploader = (*Client)(nil)
var _ Uploader = NoopUploader{}
