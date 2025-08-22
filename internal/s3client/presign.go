package s3client

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (c *Client) PresignGet(ctx context.Context, bucket, key string, expire time.Duration) (string, error) {
	ps := s3.NewPresignClient(c.S3)
	out, err := ps.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expire))
	if err != nil {
		return "", fmt.Errorf("ошибка при формировании ссылки: %w", err)
	}
	return out.URL, nil
}

func (c *Client) PresignPut(ctx context.Context, bucket, key, contentType string, expire time.Duration) (string, error) {
	ps := s3.NewPresignClient(c.S3)
	in := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	out, err := ps.PresignPutObject(ctx, in, s3.WithPresignExpires(expire))
	if err != nil {
		return "", fmt.Errorf("ошибка при формировании ссылки: %w", err)
	}
	return out.URL, nil
}
