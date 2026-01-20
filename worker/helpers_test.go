package worker

import (
	"context"
	"log/slog"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FrogoAI/testutils"
)

func TestFunctionWithTimeout(t *testing.T) {
	goroutineNumBefore := runtime.NumGoroutine()

	fn := FunctionWithTimeout(time.Millisecond, func(ctx context.Context) error {
		t := time.NewTicker(time.Millisecond)

		for {
			select {
			case <-t.C:
				slog.Default().Info("new tick")
			case <-ctx.Done():
				slog.Default().Info("context finish")
				return nil
			}
		}
	})
	testutils.Equal(t, fn(context.Background()), ErrFunctionTimeout)

	time.Sleep(time.Millisecond) // Wait for func result sent to the buffered channel

	goroutineNumAfter := runtime.NumGoroutine()
	testutils.Equal(t, goroutineNumAfter, goroutineNumBefore)

	fn = FunctionWithTimeout(time.Second, func(ctx context.Context) error {
		t := time.NewTimer(time.Millisecond)

		for {
			select {
			case <-t.C:
				slog.Default().Info("new tick")
				return nil
			case <-ctx.Done():
				slog.Default().Info("context finish")
				return nil
			}
		}
	})
	testutils.Equal(t, fn(context.Background()), nil)
}

func TestRunSyncMultipleWorkers(t *testing.T) {
	ctx := context.Background()
	ch := make(chan int, 100)

	var count int32

	go func(ch chan int) {
		for i := 0; i < 100; i++ {
			ch <- i
		}

		close(ch)
	}(ch)

	RunSyncMultipleWorkers(ctx, 4, func(_ context.Context) {
		for range ch {
			atomic.AddInt32(&count, 1)
		}
	})

	testutils.Equal(t, atomic.LoadInt32(&count), int32(100))
}

func TestRunAsyncMultipleWorkers(t *testing.T) {
	ctx := context.Background()
	in := make(chan int, 100)

	go func() {
		for i := 0; i < 100; i++ {
			in <- i
		}

		close(in)
	}()

	out := RunAsyncMultipleWorkers(ctx, 4, 100, func(_ context.Context, ch chan<- interface{}) {
		for v := range in {
			ch <- v
		}
	})

	var count int32
	for range out {
		atomic.AddInt32(&count, 1)
	}

	testutils.Equal(t, count, atomic.LoadInt32(&count))
}

func TestFanInOut(t *testing.T) {
	ctx := context.Background()
	in := make(chan interface{}, 100)

	go func() {
		for i := 0; i < 1; i++ {
			in <- i
		}

		close(in)
	}()

	var c int32

	FanInOut(ctx, 4, 100, in, func(_ context.Context, _ interface{}) {
		atomic.AddInt32(&c, 1)
	})
	testutils.Equal(t, atomic.LoadInt32(&c), int32(4))
}
