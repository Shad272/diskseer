package collect

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelIsBoundedAndCancellationStopsScheduling(t *testing.T) {
	var active, peak, count int32
	parallel(context.Background(), 20, 2, func(int) {
		n := atomic.AddInt32(&active, 1)
		for old := atomic.LoadInt32(&peak); n > old; old = atomic.LoadInt32(&peak) {
			if atomic.CompareAndSwapInt32(&peak, old, n) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		atomic.AddInt32(&active, -1)
		atomic.AddInt32(&count, 1)
	})
	if peak > 2 || count != 20 {
		t.Fatalf("peak=%d jobs=%d", peak, count)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	parallel(ctx, 1000, 4, func(int) { t.Error("cancelled job ran") })
}

func TestCollectorOutputIsBounded(t *testing.T) {
	b := boundedBuffer{limit: 10}
	if _, err := b.Write(make([]byte, 9)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write(make([]byte, 2)); err == nil || b.Len() > 10 {
		t.Fatal("output limit ignored")
	}
}

func BenchmarkBoundedCollection(b *testing.B) {
	for i := 0; i < b.N; i++ {
		parallel(context.Background(), 64, 4, func(int) {})
	}
}
