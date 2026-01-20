package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	pkgErr "github.com/pkg/errors"
)

var ErrFunctionTimeout = errors.New("function timeout")

// FunctionWithTimeout function which return result after timeout
// (be careful not release resources without using context)
func FunctionWithTimeout(
	timeout time.Duration,
	fn ExecuteFunc,
) ExecuteFunc {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		ch := make(chan error, 1)

		go func() {
			ch <- fn(ctx)
		}()

		select {
		case <-ctx.Done():
			return ErrFunctionTimeout
		case err := <-ch:
			return err
		}
	}
}

var ErrPanic = errors.New("error panic")

func RecoverWrapper(ctx context.Context, next ExecuteFunc) (err error) {
	defer func() {
		if rval := recover(); rval != nil {
			slog.Default().Error("Recovered request panic", "rval", rval)

			if v, ok := rval.(error); ok {
				err = pkgErr.WithStack(v)
			} else {
				err = pkgErr.WithStack(ErrPanic)
			}
		}
	}()

	err = next(ctx)

	return
}

// RunSyncMultipleWorkers run sync multiple workers
func RunSyncMultipleWorkers(ctx context.Context, goroutines int, fn func(ctx context.Context)) {
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			fn(ctx)
		}()
	}

	wg.Wait()
}

// RunAsyncMultipleWorkers run async multiple workers and return channel to control them
func RunAsyncMultipleWorkers(
	ctx context.Context,
	goroutines int,
	buffer int,
	fn func(context.Context, chan<- interface{}),
) <-chan interface{} {
	ch := make(chan interface{}, buffer)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			fn(ctx, ch)
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	return ch
}

// MergeChannels merge multiple channels into one
func MergeChannels(input ...<-chan interface{}) <-chan interface{} {
	out := make(chan interface{})

	var wg sync.WaitGroup
	wg.Add(len(input))

	for _, ch := range input {
		go func(ch <-chan interface{}) {
			for v := range ch {
				out <- v
			}

			wg.Done()
		}(ch)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// GetMessageOrTimeout get message or timeout
func GetMessageOrTimeout(timeout time.Duration, msg chan []byte, def []byte) []byte {
	timer := time.NewTimer(timeout)
	select {
	case <-timer.C:
		return def
	case m := <-msg:
		return m
	}
}

func FanInOut(
	ctx context.Context,
	goroutines int,
	buffer int,
	in chan interface{},
	fn func(context.Context, interface{}),
) {
	goroutinesList := make([]chan interface{}, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		goroutinesList[i] = make(chan interface{}, buffer)
		go func(i int) {
			defer wg.Done()

			for v := range goroutinesList[i] {
				fn(ctx, v)
			}
		}(i)
	}

	for v := range in {
		for _, gch := range goroutinesList {
			gch <- v
		}
	}

	for _, ch := range goroutinesList {
		close(ch)
	}

	wg.Wait()
}
