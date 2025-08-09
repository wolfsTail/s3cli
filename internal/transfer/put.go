package transfer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/schollz/progressbar/v3"
)

type PutStats struct {
	TotalFiles int
	Uploaded   int
	Failed     int
}

func UploadFile(ctx context.Context, s3c *s3.Client, bucket, key, localPath string, showProgress bool) error {
	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("не удалось открыть файл %q: %w", localPath, err)
	}
	defer f.Close()

	var body io.Reader = f
	if showProgress {
		fi, _ := f.Stat()
		bar := progressbar.NewOptions64(
			fi.Size(),
			progressbar.OptionSetDescription(fmt.Sprintf("PUT %s", filepath.Base(localPath))),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowBytes(true),
			progressbar.OptionThrottle(100e6),
			progressbar.OptionClearOnFinish(),
			progressbar.OptionSetWidth(20),
		)
		body = io.TeeReader(f, bar)
	}

	up := manager.NewUploader(s3c)
	_, err = up.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   body,
	})
	if err != nil {
		return fmt.Errorf("ошибка загрузки %q -> s3://%s/%s: %w", localPath, bucket, key, err)
	}
	return nil
}

func UploadTree(ctx context.Context, s3c *s3.Client, bucket, prefix, localDir string, jobs int, showProgress bool) (PutStats, error) {
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	type job struct {
		Local string
		Key   string
	}

	var files []string
	err := filepath.WalkDir(localDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return PutStats{}, fmt.Errorf("ошибка обхода каталога %q: %w", localDir, err)
	}

	rootAbs, _ := filepath.Abs(localDir)
	jobsCh := make(chan job, len(files))
	resCh := make(chan error, len(files))

	for _, fpath := range files {
		abs, _ := filepath.Abs(fpath)
		rel, _ := filepath.Rel(rootAbs, abs)
		rel = filepath.ToSlash(rel)
		key := prefix + rel
		jobsCh <- job{Local: fpath, Key: key}
	}
	close(jobsCh)

	var bar *progressbar.ProgressBar
	if showProgress {
		bar = progressbar.NewOptions(
			len(files),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionSetDescription("PUT (files)"),
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
			up := manager.NewUploader(s3c)
			for j := range jobsCh {
				f, err := os.Open(j.Local)
				if err != nil {
					resCh <- fmt.Errorf("не удалось открыть %q: %w", j.Local, err)
					if bar != nil {
						_ = bar.Add(1)
					}
					continue
				}
				_, err = up.Upload(ctx, &s3.PutObjectInput{
					Bucket: aws.String(bucket),
					Key:    aws.String(j.Key),
					Body:   io.Reader(f),
				})
				_ = f.Close()
				if err != nil {
					resCh <- fmt.Errorf("ошибка загрузки %q -> %s: %w", j.Local, j.Key, err)
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

	stats := PutStats{TotalFiles: len(files)}
	for err := range resCh {
		if err != nil {
			stats.Failed++
		} else {
			stats.Uploaded++
		}
	}
	return stats, nil
}
