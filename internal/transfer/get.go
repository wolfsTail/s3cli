package transfer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/schollz/progressbar/v3"
)

type GetStats struct {
	TotalFiles int
	Downloaded int
	Failed     int
}

type progressWriterAt struct {
	f   *os.File
	bar *progressbar.ProgressBar
}

func (p *progressWriterAt) WriteAt(b []byte, off int64) (int, error) {
	n, err := p.f.WriteAt(b, off)
	if p.bar != nil && n > 0 {
		_ = p.bar.Add(n)
	}
	return n, err
}

func DownloadFile(ctx context.Context, s3c *s3.Client, bucket, key, localPath string, showProgress bool) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return fmt.Errorf("не удалось создать каталог %q: %w", filepath.Dir(localPath), err)
	}
	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("не удалось создать файл %q: %w", localPath, err)
	}
	defer f.Close()

	var pw *progressWriterAt
	var bar *progressbar.ProgressBar

	if showProgress {
		head, herr := s3c.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		var total int64 = -1
		if herr == nil {
			total = aws.ToInt64(head.ContentLength)
		}
		if total > 0 {
			bar = progressbar.NewOptions64(
				total,
				progressbar.OptionSetDescription(fmt.Sprintf("GET %s", filepath.Base(localPath))),
				progressbar.OptionSetWriter(os.Stderr),
				progressbar.OptionShowBytes(true),
				progressbar.OptionThrottle(100e6),
				progressbar.OptionClearOnFinish(),
				progressbar.OptionSetWidth(20),
			)
		} else {
			bar = progressbar.NewOptions(
				-1,
				progressbar.OptionSetDescription(fmt.Sprintf("GET %s", filepath.Base(localPath))),
				progressbar.OptionSetWriter(os.Stderr),
				progressbar.OptionClearOnFinish(),
				progressbar.OptionSetWidth(20),
			)
		}
		pw = &progressWriterAt{f: f, bar: bar}
	} else {
		pw = &progressWriterAt{f: f, bar: nil}
	}

	dl := manager.NewDownloader(s3c)
	_, err = dl.Download(ctx, pw, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("ошибка скачивания s3://%s/%s -> %q: %w", bucket, key, localPath, err)
	}
	return nil
}

func DownloadKeys(ctx context.Context, s3c *s3.Client, bucket string, keys []string, prefix, localRoot string, jobs int, showProgress bool) (GetStats, error) {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	type job struct {
		Key  string
		Path string
	}

	var js []job
	for _, k := range keys {
		rel := strings.TrimPrefix(k, prefix)
		rel = filepath.FromSlash(rel)
		lp := filepath.Join(localRoot, rel)
		js = append(js, job{Key: k, Path: lp})
	}

	jobsCh := make(chan job, len(js))
	resCh := make(chan error, len(js))
	for _, j := range js {
		jobsCh <- j
	}
	close(jobsCh)

	var bar *progressbar.ProgressBar
	if showProgress {
		bar = progressbar.NewOptions(
			len(js),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionSetDescription("GET (files)"),
			progressbar.OptionShowCount(),
			progressbar.OptionClearOnFinish(),
			progressbar.OptionSetWidth(20),
		)
	}

	var wg sync.WaitGroup
	if jobs <= 0 {
		jobs = 1
	}
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dl := manager.NewDownloader(s3c)
			for j := range jobsCh {
				if err := os.MkdirAll(filepath.Dir(j.Path), 0o755); err != nil {
					resCh <- fmt.Errorf("не удалось создать каталог %q: %w", filepath.Dir(j.Path), err)
					if bar != nil {
						_ = bar.Add(1)
					}
					continue
				}
				f, err := os.Create(j.Path)
				if err != nil {
					resCh <- fmt.Errorf("не удалось создать файл %q: %w", j.Path, err)
					if bar != nil {
						_ = bar.Add(1)
					}
					continue
				}
				pw := &progressWriterAt{f: f, bar: nil} // для пачек показываем бар по количеству
				_, err = dl.Download(ctx, pw, &s3.GetObjectInput{
					Bucket: aws.String(bucket),
					Key:    aws.String(j.Key),
				})
				_ = f.Close()
				if err != nil {
					resCh <- fmt.Errorf("ошибка скачивания %s -> %q: %w", j.Key, j.Path, err)
				} else {
					resCh <- nil
				}
				if bar != nil {
					_ = bar.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	close(resCh)

	stats := GetStats{TotalFiles: len(js)}
	for err := range resCh {
		if err != nil {
			stats.Failed++
		} else {
			stats.Downloaded++
		}
	}
	return stats, nil
}
