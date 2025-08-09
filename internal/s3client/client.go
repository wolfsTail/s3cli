package s3client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsCfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/wolfsTail/s3cli/internal/config"
)

type Client struct {
	S3 *s3.Client
}

func New(ctx context.Context, a config.Alias) (*Client, error) {
	if a.Endpoint == "" {
		return nil, fmt.Errorf("ошибка: пустой endpoint в алиасе")
	}

	// http/https
	endpoint := a.Endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if a.Secure {
			endpoint = "https://" + endpoint
		} else {
			endpoint = "http://" + endpoint
		}
	}
	if _, err := url.Parse(endpoint); err != nil {
		return nil, fmt.Errorf("ошибка: некорректный endpoint %q: %w", endpoint, err)
	}

	region := nonEmpty(a.Region, "us-east-1")

	cfg, err := awsCfg.LoadDefaultConfig(ctx,
		awsCfg.WithRegion(region),
		awsCfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(a.AccessKey, a.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("не удалось инициализировать конфигурацию AWS SDK: %w", err)
	}

	httpClient := &http.Client{Timeout: 2 * time.Minute}

	s3c := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.Region = region
		o.UsePathStyle = a.PathStyle
		o.BaseEndpoint = aws.String(endpoint) // <-- ключевой момент: кастомный S3-совместимый endpoint
		o.HTTPClient = httpClient
	})

	return &Client{S3: s3c}, nil
}

func nonEmpty(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
