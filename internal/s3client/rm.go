package s3client

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.S3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("ошибка удаления объекта: %w", err)
	}

	waiter := s3.NewObjectNotExistsWaiter(c.S3)
	_ = waiter.Wait(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, 30*time.Second)

	return nil
}

func (c *Client) DeletePrefix(ctx context.Context, bucket, prefix string) (int, error) {
	p := s3.NewListObjectsV2Paginator(c.S3, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}, func(o *s3.ListObjectsV2PaginatorOptions) { o.Limit = 1000 })

	total := 0
	batch := make([]types.ObjectIdentifier, 0, 1000)

	flush := func() (int, error) {
		if len(batch) == 0 {
			return 0, nil
		}
		out, err := c.S3.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &types.Delete{
				Objects: batch,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			return 0, fmt.Errorf("ошибка пакетного удаления: %w", err)
		}
		n := len(out.Deleted)
		total += n
		batch = batch[:0]
		return n, nil
	}

	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return total, fmt.Errorf("ошибка листинга перед удалением: %w", err)
		}
		for _, it := range out.Contents {
			if it.Key == nil {
				continue
			}
			batch = append(batch, types.ObjectIdentifier{Key: it.Key})
			if len(batch) == 1000 {
				if _, err := flush(); err != nil {
					return total, err
				}
			}
		}
	}
	if _, err := flush(); err != nil {
		return total, err
	}
	return total, nil
}
