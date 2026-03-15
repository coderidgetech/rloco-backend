package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StorageService interface {
	Upload(ctx context.Context, file io.Reader, filename string, contentType string) (string, error)
	Delete(ctx context.Context, filename string) error
	GetURL(ctx context.Context, filename string) (string, error)
}

type storageService struct {
	storageType      string
	storageEndpoint  string
	storageAccessKey string
	storageSecretKey string
	storageBucket    string
	storagePublicURL string
	localPath       string
	s3Client        *s3.Client
}

// NewStorageService creates a storage service. For S3/R2 use storageType "s3", endpoint e.g. https://<ACCOUNT_ID>.r2.cloudflarestorage.com; publicURL is optional (e.g. https://pub-xxx.r2.dev).
func NewStorageService(storageType, endpoint, accessKey, secretKey, bucket, publicURL string) StorageService {
	localPath := "./uploads"
	if storageType == "local" {
		os.MkdirAll(localPath, 0755)
	}

	svc := &storageService{
		storageType:      storageType,
		storageEndpoint:  endpoint,
		storageAccessKey: accessKey,
		storageSecretKey: secretKey,
		storageBucket:    bucket,
		storagePublicURL: strings.TrimSuffix(publicURL, "/"),
		localPath:        localPath,
	}

	if storageType == "s3" || storageType == "r2" {
		ep := endpoint
		if !strings.HasPrefix(ep, "http") {
			ep = "https://" + ep
		}
		creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
		svc.s3Client = s3.New(s3.Options{
			Region:       "auto",
			Credentials:  creds,
			BaseEndpoint: aws.String(ep),
			UsePathStyle: true,
		})
	}

	return svc
}

func (s *storageService) Upload(ctx context.Context, file io.Reader, filename string, contentType string) (string, error) {
	ext := filepath.Ext(filename)
	newFilename := fmt.Sprintf("%s_%d%s", primitive.NewObjectID().Hex(), time.Now().Unix(), ext)

	if s.storageType == "local" {
		filePath := filepath.Join(s.localPath, newFilename)
		dst, err := os.Create(filePath)
		if err != nil {
			return "", err
		}
		defer dst.Close()
		if _, err := io.Copy(dst, file); err != nil {
			return "", err
		}
		return fmt.Sprintf("/uploads/%s", newFilename), nil
	}

	if s.s3Client != nil {
		_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(s.storageBucket),
			Key:         aws.String(newFilename),
			Body:        file,
			ContentType: aws.String(contentType),
		})
		if err != nil {
			return "", err
		}
		return newFilename, nil
	}

	return newFilename, nil
}

func (s *storageService) Delete(ctx context.Context, filename string) error {
	if s.storageType == "local" {
		filePath := filepath.Join(s.localPath, filepath.Base(filename))
		return os.Remove(filePath)
	}
	if s.s3Client != nil {
		key := filepath.Base(filename)
		_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.storageBucket),
			Key:    aws.String(key),
		})
		return err
	}
	return nil
}

func (s *storageService) GetURL(ctx context.Context, filename string) (string, error) {
	if s.storageType == "local" {
		return filename, nil
	}
	key := filepath.Base(filename)
	if s.storagePublicURL != "" {
		return fmt.Sprintf("%s/%s", s.storagePublicURL, key), nil
	}
	return fmt.Sprintf("https://%s/%s/%s", s.storageEndpoint, s.storageBucket, key), nil
}

