package s3client

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ObjectStat struct {
	Key          string
	Size         int64
	LastModified string
	ETag         string
	ContentType  string
}

// StatObject — получить метаданные
func (c *Client) StatObject(ctx context.Context, bucket, key string) (*ObjectStat, error) {
	out, err := c.S3.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка получения метаданных: %w", err)
	}
	return &ObjectStat{
		Key:          key,
		Size:         aws.ToInt64(out.ContentLength),
		LastModified: aws.ToTime(out.LastModified).Format("2025-01-02 15:20"),
		ETag:         aws.ToString(out.ETag),
		ContentType:  aws.ToString(out.ContentType),
	}, nil
}

// CatObject — вывести объект(stdout).
func (c *Client) CatObject(ctx context.Context, bucket, key string, w io.Writer) error {
	out, err := c.S3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("ошибка чтения объекта: %w", err)
	}
	defer out.Body.Close()

	_, err = io.Copy(w, out.Body)
	return err
}
