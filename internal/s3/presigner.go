package s3

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"sql-query/internal/config"
)

// Presigner generates presigned URLs for S3-compatible storage.
type Presigner struct {
	client        *s3.Client
	presignClient *s3.PresignClient
}

// NewPresigner creates a Presigner from the application config.
func NewPresigner(cfg *config.Config) (*Presigner, error) {
	if !cfg.HasS3Config() {
		return nil, fmt.Errorf("S3 配置不完整：需要 S3_ACCESS_KEY, S3_SECRET_KEY, S3_REGION")
	}

	creds := credentials.NewStaticCredentialsProvider(
		cfg.S3AccessKey,
		cfg.S3SecretKey,
		"",
	)

	opts := []func(*s3.Options){
		func(o *s3.Options) {
			o.Region = cfg.S3Region
			o.Credentials = creds
		},
	}

	if cfg.S3Endpoint != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.S3Endpoint)
			o.UsePathStyle = true
		})
	}

	client := s3.New(s3.Options{}, opts...)
	presignClient := s3.NewPresignClient(client)

	return &Presigner{
		client:        client,
		presignClient: presignClient,
	}, nil
}

// ParseBucketKey splits "bucket:key" into bucket and key.
func ParseBucketKey(value string) (bucket, key string, err error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("无效的 S3 路径格式，期望 'bucket:key'，实际: %s", value)
	}
	return parts[0], parts[1], nil
}

// GeneratePresignedURL creates a presigned GET URL.
// If download is true, sets Content-Disposition to trigger browser download.
func (p *Presigner) GeneratePresignedURL(bucket, key string, expiry time.Duration, download bool) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	if download {
		filename := key
		if idx := strings.LastIndex(key, "/"); idx != -1 {
			filename = key[idx+1:]
		}
		disposition := fmt.Sprintf("attachment; filename=\"%s\"", filename)
		input.ResponseContentDisposition = aws.String(disposition)
	}

	req, err := p.presignClient.PresignGetObject(context.Background(), input, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

// SignValue presigns a single "bucket:key" value.
// expiryStr is a Go duration string like "24h", "15m", "1h30m".
func (p *Presigner) SignValue(value, expiryStr string, download bool) (string, error) {
	bucket, key, err := ParseBucketKey(value)
	if err != nil {
		return "", err
	}

	expiry, err := time.ParseDuration(expiryStr)
	if err != nil {
		return "", fmt.Errorf("无效的过期时间格式: %s", expiryStr)
	}

	return p.GeneratePresignedURL(bucket, key, expiry, download)
}
