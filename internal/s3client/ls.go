package s3client

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ObjectInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
}

func (c *Client) ListOneLevel(ctx context.Context, bucket, prefix string) ([]string, []ObjectInfo, error) {
	var commonPrefixes []string
	var objects []ObjectInfo

	delimiter := "/"
	input := &s3.ListObjectsV2Input{
		Bucket:    &bucket,
		Prefix:    &prefix,
		Delimiter: &delimiter,
	}

	p := s3.NewListObjectsV2Paginator(c.S3, input, func(o *s3.ListObjectsV2PaginatorOptions) {
		o.Limit = 1000
	})

	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("ошибка листинга: %w", err)
		}
		for _, cp := range out.CommonPrefixes {
			if cp.Prefix != nil {
				commonPrefixes = append(commonPrefixes, *cp.Prefix)
			}
		}
		for _, it := range out.Contents {
			if it.Key == nil {
				continue
			}
			if prefix != "" && *it.Key == prefix {
				continue
			}
			objects = append(objects, ObjectInfo{
				Key:          *it.Key,
				Size:         aws.ToInt64(it.Size),
				LastModified: derefTime(it.LastModified),
			})
		}
	}

	return commonPrefixes, objects, nil
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
