package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/FrogoAI/testutils"
)

func TestPoolExecuteSameError(t *testing.T) {
	defer func() {
		if rval := recover(); rval != nil {
			slog.Default().Error("recover", "rval", rval)
		}
	}()

	ctx := context.Background()
	err := errors.New("test error")
	pool := NewPool(ctx, RecoverWrapper)

	pool.Execute(func(_ context.Context) error {
		return fmt.Errorf("error: %w", err)
	})
	pool.Execute(func(_ context.Context) error {
		return fmt.Errorf("error: %w", err)
	})
	pool.Execute(func(_ context.Context) error {
		return fmt.Errorf("error: %w", err)
	})

	poolErr := pool.Wait()
	testutils.Equal(t, poolErr, err)
	testutils.Equal(t, poolErr.Error(), "error: test error\nerror: test error\nerror: test error")
}

func TestPoolExecuteNoError(t *testing.T) {
	defer func() {
		if rval := recover(); rval != nil {
			slog.Default().Error("recover", "rval", rval)
		}
	}()

	ctx := context.Background()
	pool := NewPool(ctx, RecoverWrapper)

	pool.Execute(func(_ context.Context) error {
		return nil
	})
	pool.Execute(func(_ context.Context) error {
		return nil
	})
	pool.Execute(func(_ context.Context) error {
		return nil
	})

	poolErr := pool.Wait()
	testutils.Equal(t, poolErr, nil)
}

func TestPool(t *testing.T) {
	err := errors.New("test")
	err2 := errors.New("test2")
	err3 := errors.New("test3")

	var c int32

	ctx := context.Background()
	p := NewPool(ctx)
	p.Execute(func(context.Context) error {
		atomic.AddInt32(&c, 1)
		return nil
	})
	p.Execute(func(context.Context) error {
		atomic.AddInt32(&c, 1)
		return nil
	})
	p.Execute(func(context.Context) error {
		return err
	})
	p.Execute(func(context.Context) error {
		return err2
	})
	resErr := p.Wait()

	testutils.Equal(t, atomic.LoadInt32(&c), int32(2))
	testutils.Equal(t, errors.Is(resErr, err), true)
	testutils.Equal(t, errors.Is(resErr, err2), true)
	testutils.Equal(t, errors.Is(resErr, err3), false)
}

func TestPoolCloseContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var c int32

	p := NewPool(ctx)
	p.Execute(func(context.Context) error {
		atomic.AddInt32(&c, 1)
		return nil
	})
	p.Execute(func(context.Context) error {
		atomic.AddInt32(&c, 1)
		return nil
	})

	err := p.Wait()

	testutils.Equal(t, atomic.LoadInt32(&c), int32(2))
	testutils.Equal(t, err, nil)
}

func TestPoolNilFunction(t *testing.T) {
	p := NewPool(context.Background())
	p.Execute(nil)

	err := p.Wait()
	testutils.Equal(t, err, ErrNilFunction)
}
