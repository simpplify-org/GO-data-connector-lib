package bucket

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ToS3 struct {
	client     *s3.Client
	BucketName string
}

func NewToS3(AwsAccessKey, AwsSecretKey, AwsRegion, BucketName string, isTest bool) (*ToS3, error) {
	var cfg aws.Config
	var err error

	if isTest {
		endpoint := "http://localhost:4566"
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(AwsRegion),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(AwsAccessKey, AwsSecretKey, "")),
			config.WithEndpointResolver(aws.EndpointResolverFunc(
				func(service, region string) (aws.Endpoint, error) {
					return aws.Endpoint{
						URL:           endpoint,
						SigningRegion: AwsRegion,
					}, nil
				},
			)),
		)
	} else {
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(AwsRegion),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(AwsAccessKey, AwsSecretKey, "")),
		)
	}

	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if isTest {
			o.UsePathStyle = true
		}
	})

	return &ToS3{
		client:     client,
		BucketName: BucketName,
	}, nil
}

func (b *ToS3) CreateBucket(ctx context.Context) error {
	_, err := b.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(b.BucketName),
	})
	if err != nil {
		return err
	}

	fmt.Println("Bucket criado:", b.BucketName)
	return nil
}

func (b *ToS3) UploadFile(ctx context.Context, key, filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	_, err = b.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(b.BucketName),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return err
	}

	fmt.Println("Arquivo enviado para:", key)
	return nil
}

func (b *ToS3) DownloadFile(ctx context.Context, key, filePath string) error {
	resp, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	fmt.Println("Arquivo baixado para:", filePath)
	return nil
}

func (b *ToS3) DeleteFile(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return err
	}

	fmt.Println("Arquivo deletado:", key)
	return nil
}

func (b *ToS3) DeleteBucket(ctx context.Context) error {
	_, err := b.client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(b.BucketName),
	})
	if err != nil {
		return err
	}

	fmt.Println("Bucket deletado:", b.BucketName)
	return nil
}
