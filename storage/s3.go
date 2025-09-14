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

func S3FileGet(path string, download bool) (GetResult, error) {
	if download {
		//download file
		data, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(os.Getenv("S3_BUCKET")),
			Key:    aws.String(path),
		})
		if err != nil {
			return GetResult{}, err
		}
		defer data.Body.Close()
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(data.Body)
		if err != nil {
			return GetResult{}, err
		}
		bytesData := buf.Bytes()
		return GetResult{Data: &bytesData}, nil
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
			return GetResult{}, err
		}
		return GetResult{URL: &presignedReq.URL}, nil
	}
}

func S3FilePut(path string, data []byte) error {
	_, err := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Key:    aws.String(path),
		Body:   bytes.NewReader(data),
	})
	return err
}

func S3FileDelete(path string) error {
	_, err := s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Key:    aws.String(path),
	})
	return err
}

func S3DirectoryDelete(path string) error {
	_, err := s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Key:    aws.String(path),
	})
	return err
}

func S3DirectoryListing(path string, recursive bool, includeFolders bool) ([]string, error) {
	var result []string
	dirSet := make(map[string]struct{})
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(os.Getenv("S3_BUCKET")),
		Prefix: aws.String(path),
	}
	if !recursive {
		input.Delimiter = aws.String("/")
	}
	paginator := s3.NewListObjectsV2Paginator(s3Client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, err
		}
		// Always add folders from CommonPrefixes if includeFolders is true
		if includeFolders {
			for _, prefix := range page.CommonPrefixes {
				println(*prefix.Prefix)
				if prefix.Prefix != nil {
					dir := *prefix.Prefix
					if _, exists := dirSet[dir]; !exists {
						dirSet[dir] = struct{}{}
						result = append(result, dir)
						println("Added folder:", dir)
					}
				}
			}
		}
		for _, file := range page.Contents {
			if file.Key != nil {
				key := *file.Key
				// Only add files (not folders) if includeFolders is true and key is not a folder
				if includeFolders {
					if idx := indexOfSlash(key, len(path)); idx != -1 {
						dir := key[:idx]
						// Already handled by CommonPrefixes, skip
						if _, exists := dirSet[dir]; exists {
							continue
						}
					}
				}
				result = append(result, key)
			}
		}
	}
	return result, nil
}

// indexOfSlash returns the index of the first '/' after the prefix length, or -1 if not found
func indexOfSlash(s string, prefixLen int) int {
	for i := prefixLen; i < len(s); i++ {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}
