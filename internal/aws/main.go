package aws

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
)

type S3Service struct {
	client     *s3.Client
	bucketName string
	region     string
	baseUrl    string
}

func NewS3Service(bucketName, region string) (*S3Service, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithClientLogMode(aws.LogRetries | aws.LogRequestWithBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg)

	baseUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com", bucketName, region)

	return &S3Service{
		client:     client,
		bucketName: bucketName,
		region:     region,
		baseUrl:    baseUrl,
	}, nil
}

// NewS3ServiceFromEnv constructs S3Service using environment variables.
// Required envs: S3_BUCKET_NAME, S3_REGION, S3_ACCESS_KEY_ID, S3_SECRET_ACCESS_KEY
// Optional: S3_FOLDER_NAME (returned for convenience)
func NewS3ServiceFromEnv() (*S3Service, string, error) {
	// Load .env if present
	_ = godotenv.Load()

	bucket := strings.TrimSpace(os.Getenv("S3_BUCKET_NAME"))
	region := strings.TrimSpace(os.Getenv("S3_REGION"))
	accessKey := strings.TrimSpace(os.Getenv("S3_ACCESS_KEY_ID"))
	secretKey := strings.TrimSpace(os.Getenv("S3_SECRET_ACCESS_KEY"))
	folder := strings.TrimSpace(os.Getenv("S3_FOLDER_NAME"))

	if bucket == "" || region == "" || accessKey == "" || secretKey == "" {
		return nil, "", fmt.Errorf("missing one or more required S3 envs: S3_BUCKET_NAME, S3_REGION, S3_ACCESS_KEY_ID, S3_SECRET_ACCESS_KEY")
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""))),
		config.WithClientLogMode(aws.LogRetries|aws.LogRequestWithBody),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load AWS config: %v", err)
	}

	client := s3.NewFromConfig(cfg)
	baseUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com", bucket, region)

	svc := &S3Service{
		client:     client,
		bucketName: bucket,
		region:     region,
		baseUrl:    baseUrl,
	}
	return svc, folder, nil
}

// UploadFile uploads file to S3 and returns presigned URL with TTL
func (s *S3Service) UploadFile(
	ctx context.Context,
	key string,
	fileData []byte,
	mimeType string,
	ttlMinutes int,
) (presignedUrl string, err error) {
	log.Printf(" Uploading to S3: s3://%s/%s (size: %.2f MB)",
		s.bucketName, key, float64(len(fileData))/1024/1024)

	// Upload file
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(fileData),
		ContentType: aws.String(mimeType),
		// Auto-delete via lifecycle policy is handled in S3 bucket config
		Metadata: map[string]string{
			"uploadTime": time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %v", err)
	}

	log.Printf(" File uploaded successfully: %s", key)

	// Generate presigned URL
	presignedUrl, err = s.GetPresignedUrl(ctx, key, ttlMinutes)
	if err != nil {
		return "", err
	}

	return presignedUrl, nil
}

// GetPresignedUrl generates a presigned URL for downloading the file
func (s *S3Service) GetPresignedUrl(
	ctx context.Context,
	key string,
	ttlMinutes int,
) (string, error) {
	presignClient := s3.NewPresignFromClient(s.client)

	request, err := presignClient.PresignGetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(s.bucketName),
			Key:    aws.String(key),
		},
		func(opts *s3.PresignOptions) {
			opts.Expires = time.Duration(ttlMinutes) * time.Minute
		},
	)

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %v", err)
	}

	log.Printf(" Presigned URL generated (expires in %d min): %s",
		ttlMinutes, request.URL[:80]+"...")

	return request.URL, nil
}

// DeleteFile deletes a file from S3
func (s *S3Service) DeleteFile(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %v", err)
	}

	log.Printf(" File deleted from S3: %s", key)
	return nil
}