package collect

import (
	"bytes"
	"context"
	"errors"
	"sync"
)

type boundedBuffer struct {
	bytes.Buffer
	limit int
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.limit {
		return 0, errors.New("collector output exceeds memory limit")
	}
	return b.Buffer.Write(p)
}

// Each disk owns one job; no concurrent requests to the same device. Work is
// bounded independently of the number of installed disks and stops scheduling
// as soon as cancellation is requested. In-flight kernel IO must finish first.
func parallel(ctx context.Context, count, workers int, read func(int)) {
	if workers < 1 {
		workers = 1
	}
	if workers > 4 {
		workers = 4
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for n := 0; n < workers; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if ctx.Err() == nil {
					read(i)
				}
			}
		}()
	}
send:
	for i := 0; i < count; i++ {
		select {
		case <-ctx.Done():
			break send
		case jobs <- i:
		}
	}
	close(jobs)
	wg.Wait()
}
