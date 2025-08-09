package cli

import (
	"fmt"
	"strings"
)

type s3Path struct {
	Alias  string
	Bucket string
	Key    string // может быть пустым
}

func parseS3Path(raw string) (s3Path, error) {
	p := raw
	if strings.HasPrefix(p, "s3://") {
		p = strings.TrimPrefix(p, "s3://")
	}
	parts := strings.Split(p, "/")
	if len(parts) < 2 {
		return s3Path{}, fmt.Errorf("ошибка: ожидаю формат alias/bucket[/key], получено: %q", raw)
	}
	sp := s3Path{
		Alias:  parts[0],
		Bucket: parts[1],
	}
	if len(parts) > 2 {
		sp.Key = strings.Join(parts[2:], "/")
	}
	return sp, nil
}
