package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FrogoAI/testutils"
)

func TestWorker(t *testing.T) {
	ctx := context.Background()

	testcases := []struct {
		name   string
		err    error
		fn     func(ctx context.Context) error
		cancel bool
	}{
		{
			name: "nil error",
			fn: func(_ context.Context) error {
				return nil
			},
		},
		{
			name: "not nil error",
			fn: func(_ context.Context) error {
				return errors.New("test error")
			},
			err: errors.New("test error"),
		},
		{
			name: "stop",
			fn: func(ctx context.Context) error {
				t := time.NewTimer(time.Second * 1)
				select {
				case <-t.C:
					return errors.New("timeout")
				case <-ctx.Done():
					return nil
				}
			},
			cancel: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWorker(ctx, tc.fn)

			go func() {
				if tc.cancel {
					w.Cancel()
				}
			}()

			err := w.Handle()
			testutils.Equal(t, err, tc.err)
		})
	}
}

func handler(ctx context.Context) error {
	t := time.NewTimer(time.Second * 2)
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return nil
	}
}

func TestWorkersPool(t *testing.T) {
	ctx := context.Background()
	w := NewWorkersPool[string](ctx)
	w.Execute(handler)
	w.Execute(handler)
	w.Stop()

	err := w.Wait()
	testutils.Equal(t, err, nil)
}

func TestTemporalWorker(t *testing.T) {
	ctx := context.Background()
	w := NewWorkersPool[string](ctx)

	var (
		executed atomic.Bool
		timeout  atomic.Bool
	)

	ch := make(chan string, 1)

	w.Execute(func(ctx context.Context) error {
		return w.TemporalWorker(ctx, 50*time.Millisecond, func() {
			timeout.Store(true)
		}, ch, func(_ context.Context, _ string) error {
			time.Sleep(10 * time.Millisecond)
			executed.Store(true)

			return nil
		})
	})

	go func() {
		tk := time.NewTicker(10 * time.Millisecond)
		defer tk.Stop()

		tm := time.NewTimer(300 * time.Millisecond)
		defer tm.Stop()

		for {
			select {
			case <-tm.C:
				w.Stop()
				return
			case <-tk.C:
				ch <- "test string"
			}
		}
	}()

	err := w.Wait()

	testutils.Equal(t, err, nil)
	testutils.Equal(t, executed.Load(), true)
	testutils.Equal(t, timeout.Load(), false)
}

func TestTemporalWorkerStops(t *testing.T) {
	ctx := context.Background()
	w := NewWorkersPool[string](ctx)

	var (
		executed atomic.Bool
		timeout  atomic.Bool
	)

	ch := make(chan string, 1)

	w.Execute(func(ctx context.Context) error {
		return w.TemporalWorker(ctx, 50*time.Millisecond, func() {
			timeout.Store(true)
		}, ch, func(_ context.Context, _ string) error {
			time.Sleep(10 * time.Millisecond)
			executed.Store(true)

			return nil
		})
	})

	go func() {
		tm := time.NewTimer(300 * time.Millisecond)
		defer tm.Stop()

		for range tm.C {
			w.Stop()
			return
		}
	}()

	err := w.Wait()

	testutils.Equal(t, err, nil)
	testutils.Equal(t, executed.Load(), false)
	testutils.Equal(t, timeout.Load(), true)
}
