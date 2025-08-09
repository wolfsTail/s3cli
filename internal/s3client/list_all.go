package s3client

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ListAllKeys — ключи под префиксом
func (c *Client) ListAllKeys(ctx context.Context, bucket, prefix string) ([]string, error) {
	p := s3.NewListObjectsV2Paginator(c.S3, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	}, func(o *s3.ListObjectsV2PaginatorOptions) { o.Limit = 1000 })

	var keys []string
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ошибка листинга: %w", err)
		}
		for _, it := range out.Contents {
			if it.Key != nil {
				keys = append(keys, aws.ToString(it.Key))
			}
		}
	}
	return keys, nil
}
