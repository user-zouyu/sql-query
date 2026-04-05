package processor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"sql-query/internal/config"
	"sql-query/internal/log"
	"sql-query/internal/parser"
	"sql-query/internal/s3"
)

// Task represents a single cell that needs presigning.
type Task struct {
	RowIndex int
	ColIndex int
	Value    *string
	Meta     *parser.ColumnMeta
}

// Process presigns all [URL] column values concurrently.
// Returns an error immediately if any single presign fails.
func Process(cfg *config.Config, columns []*parser.Column, data [][]*string, workers int) error {
	var urlColumns []int
	for colIdx, col := range columns {
		if col.HasMeta("URL") {
			urlColumns = append(urlColumns, colIdx)
		}
	}

	if len(urlColumns) == 0 {
		return nil
	}

	if !cfg.HasS3Config() {
		return fmt.Errorf("存在 [URL] 元数据但未配置 S3：需要 S3_ACCESS_KEY, S3_SECRET_KEY, S3_REGION")
	}

	presigner, err := s3.NewPresigner(cfg)
	if err != nil {
		return fmt.Errorf("创建 S3 预签名器失败: %w", err)
	}

	// Build task list
	var tasks []Task
	for rowIdx, row := range data {
		for _, colIdx := range urlColumns {
			if row[colIdx] != nil && *row[colIdx] != "" {
				tasks = append(tasks, Task{
					RowIndex: rowIdx,
					ColIndex: colIdx,
					Value:    row[colIdx],
					Meta:     columns[colIdx].GetMeta("URL"),
				})
			}
		}
	}

	if len(tasks) == 0 {
		return nil
	}

	log.Info("发现 %d 个 URL 列需要预签名处理", len(urlColumns))
	log.Info("共 %d 个单元格需要签名", len(tasks))

	taskChan := make(chan Task, len(tasks))
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once
	var completed int64
	totalTasks := int64(len(tasks))

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case task, ok := <-taskChan:
					if !ok {
						return
					}
					err := processTask(presigner, data, task)
					if err != nil {
						errOnce.Do(func() {
							firstErr = fmt.Errorf("行 %d 列 %d: %w", task.RowIndex, task.ColIndex, err)
							cancel()
						})
						return
					}
					current := atomic.AddInt64(&completed, 1)
					if current%100 == 0 || current == totalTasks {
						log.Info("签名进度: %d/%d (%.1f%%)", current, totalTasks, float64(current)/float64(totalTasks)*100)
					}
				}
			}
		}()
	}

	wg.Wait()

	if firstErr != nil {
		return firstErr
	}

	return nil
}

func processTask(presigner *s3.Presigner, data [][]*string, task Task) error {
	expiry := "24h"
	if task.Meta != nil && task.Meta.Args["expiry"] != "" {
		expiry = task.Meta.Args["expiry"]
	}

	download := task.Meta != nil && task.Meta.Args["download"] == "true"

	signedURL, err := presigner.SignValue(*task.Value, expiry, download)
	if err != nil {
		return err
	}

	*data[task.RowIndex][task.ColIndex] = signedURL
	return nil
}
