package storage

import (
	"bytes"
	"context"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var s3Client *s3.Client

func InitS3Storage() *s3.Client {
	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		panic("S3_BUCKET environment variable is required for S3 storage mode")
	}
	region := os.Getenv("S3_REGION")
	if region == "" {
		panic("S3_REGION environment variable is required for S3 storage mode")
	}

	keyId := os.Getenv("S3_KEY_ID")
	keySecret := os.Getenv("S3_KEY_SECRET")
	if keyId == "" || keySecret == "" {
		panic("S3_KEY_ID and S3_KEY_SECRET environment variables are required for S3 storage mode")
	}

	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://s3." + region + ".amazonaws.com"
	}

	// Load config with static credentials
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(keyId, keySecret, ""),
		),
	)
	if err != nil {
		panic("failed to load AWS config: " + err.Error())
	}

	// Create S3 client (supports custom endpoint, e.g. MinIO)
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true // fix for localhost / MinIO
	})

	s3Client = client
	return s3Client
}

func S3FileExists(path string) bool {
	_, err := s3Client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Key:    aws.String(path),
	})
	return err == nil
}

func S3FileGet(path string, download bool) GetResult {
	if download {
		//download file
		data, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(os.Getenv("S3_BUCKET")),
			Key:    aws.String(path),
		})
		if err != nil {
			panic("failed to download file: " + err.Error())
		}
		defer data.Body.Close()
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(data.Body)
		if err != nil {
			panic("failed to read file body: " + err.Error())
		}
		bytesData := buf.Bytes()
		return GetResult{Data: &bytesData}
	} else {
		//generate presigned url
		presignClient := s3.NewPresignClient(s3Client)
		presignedReq, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(os.Getenv("S3_BUCKET")),
			Key:    aws.String(path),
		}, func(po *s3.PresignOptions) {
			po.Expires = time.Second * 10
		})
		if err != nil {
			panic("failed to generate presigned url: " + err.Error())
		}
		return GetResult{URL: &presignedReq.URL}
	}
}

func S3FilePut(path string, data []byte) {
	_, err := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Key:    aws.String(path),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		panic("failed to upload file: " + err.Error())
	}
}

func S3FileDelete(path string) {
	_, err := s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Key:    aws.String(path),
	})
	if err != nil {
		panic("failed to delete file: " + err.Error())
	}
}
