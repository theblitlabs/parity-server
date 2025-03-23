package services

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/theblitlabs/gologger"
)

type S3Service struct {
	client     *s3.Client
	bucketName string
}

func NewS3Service(bucketName string) (*S3Service, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	client := s3.NewFromConfig(cfg)

	return &S3Service{
		client:     client,
		bucketName: bucketName,
	}, nil
}

func (s *S3Service) UploadDockerImage(ctx context.Context, imageData []byte, imageName string) (string, error) {
	log := gologger.Get()

	// Generate a unique filename
	filename := fmt.Sprintf("%s-%s%s", imageName, uuid.New().String(), ".tar")
	key := path.Join("docker-images", filename)

	// Upload the file to S3
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucketName),
		Key:           aws.String(key),
		Body:          bytes.NewReader(imageData),
		ContentType:   aws.String("application/x-tar"),
		CacheControl:  aws.String("max-age=31536000"),
		ContentLength: aws.Int64(int64(len(imageData))),
	})

	if err != nil {
		log.Error().Err(err).
			Str("bucket", s.bucketName).
			Str("key", key).
			Msg("Failed to upload Docker image to S3")
		return "", fmt.Errorf("failed to upload Docker image: %w", err)
	}

	// Generate a pre-signed URL for the object
	presignClient := s3.NewPresignClient(s.client)
	presignedURL, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(24*time.Hour)) // URL valid for 24 hours

	if err != nil {
		log.Error().Err(err).
			Str("bucket", s.bucketName).
			Str("key", key).
			Msg("Failed to generate pre-signed URL")
		return "", fmt.Errorf("failed to generate pre-signed URL: %w", err)
	}

	log.Info().
		Str("bucket", s.bucketName).
		Str("key", key).
		Str("url", presignedURL.URL).
		Msg("Successfully uploaded Docker image to S3")

	return presignedURL.URL, nil
}

func (s *S3Service) DeleteDockerImage(ctx context.Context, imageURL string) error {
	// Extract the key from the URL
	// Note: This is a simplified example. You might need more robust URL parsing
	key := path.Base(imageURL)
	key = path.Join("docker-images", key)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
	})

	if err != nil {
		return fmt.Errorf("failed to delete Docker image: %w", err)
	}

	return nil
} 