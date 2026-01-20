package buffer

import (
	"context"
	"time"

	"github.com/FrogoAI/multiproc/worker"
)

type Delay struct {
	pool  *worker.Pool
	delay time.Duration
}

func NewDelay(ctx context.Context, delay time.Duration) *Delay {
	return &Delay{
		pool:  worker.NewPool(ctx),
		delay: delay,
	}
}

func (d *Delay) Add(processor func() error) error {
	d.pool.Execute(func(_ context.Context) error {
		time.Sleep(d.delay)
		return processor()
	})

	if d.delay == 0 {
		return d.Wait()
	}

	return nil
}

func (d *Delay) Wait() error {
	return d.pool.Wait()
}
