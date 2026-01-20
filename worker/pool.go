package worker

import (
	"context"
	"errors"
	"sync"
)

type ExecuteFunc func(ctx context.Context) error

// Pool of workers
type Pool struct {
	wg     sync.WaitGroup
	mu     sync.RWMutex
	errs   []error
	ctx    context.Context
	cancel func()

	wrapper func(ctx context.Context, next ExecuteFunc) error
}

// NewPool return new pool
func NewPool(
	ctx context.Context,
	wrappers ...func(ctx context.Context, next ExecuteFunc) error,
) *Pool {
	var wrapper func(ctx context.Context, next ExecuteFunc) error
	if len(wrappers) == 1 && wrappers[0] != nil {
		wrapper = wrappers[0]
	}

	ctx, cancel := context.WithCancel(ctx)

	return &Pool{
		wrapper: wrapper,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (p *Pool) Close() {
	p.cancel()
}

func (p *Pool) addErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.errs = append(p.errs, err)
}

func (p *Pool) getErr() error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return errors.Join(p.errs...)
}

// Execute function in gorutine
func (p *Pool) Execute(fn ExecuteFunc) {
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		err := p.execute(fn)
		if err != nil {
			p.addErr(err)
		}
	}()
}

func (p *Pool) execute(fn ExecuteFunc) error {
	if fn == nil {
		return ErrNilFunction
	}

	if p.wrapper == nil {
		return fn(p.ctx)
	}

	return p.wrapper(p.ctx, fn)
}

// Wait wait for all functions
func (p *Pool) Wait() error {
	p.wg.Wait()

	defer p.Close()

	return p.getErr()
}
