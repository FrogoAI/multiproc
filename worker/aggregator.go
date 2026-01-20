package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/FrogoAI/memory/utils"
)

type Aggregator[K any] struct {
	mu   sync.RWMutex
	ctx  context.Context
	cond *sync.Cond

	ticker     time.Duration
	maxCount   int
	goroutines int
	cancel     context.CancelFunc
	closed     bool
	counter    chan struct{}

	items     *utils.SafeList[K]
	processor func([]K) error
}

func NewAggregator[K any](
	ctx context.Context, goroutines, count int, ticker time.Duration, processor func([]K) error,
) *Aggregator[K] {
	if goroutines <= 0 {
		goroutines = 1
	}

	if count <= 0 {
		goroutines = 1
	}

	ctx, cancel := context.WithCancel(ctx)

	ag := &Aggregator[K]{
		ctx:        ctx,
		cancel:     cancel,
		maxCount:   count,
		goroutines: goroutines,
		ticker:     ticker,

		items:     utils.NewSafeList[K](),
		processor: processor,
		counter:   make(chan struct{}),
	}

	ag.cond = sync.NewCond(&ag.mu)

	return ag
}

func (w *Aggregator[K]) Add(req K) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.closed {
		return
	}

	w.items.Add(req)

	if w.items.Count() >= w.maxCount {
		w.counter <- struct{}{}
	}
}

func (w *Aggregator[K]) Process() error {
	defer w.cond.Broadcast()

	list := w.items.Reset()

	if len(list) == 0 || w.processor == nil {
		return nil
	}

	return w.processor(list)
}

func (w *Aggregator[K]) Wait() {
	w.mu.Lock()
	defer w.mu.Unlock()

	for w.Count() != 0 {
		w.cond.Wait()
	}
}

func (w *Aggregator[K]) Count() int {
	return w.items.Count()
}

func (w *Aggregator[K]) Close() {
	w.cancel()
}

func (w *Aggregator[K]) Flusher() error {
	tck := time.NewTicker(w.ticker)

	var resultErr error

	RunSyncMultipleWorkers(w.ctx, w.goroutines, func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				w.mu.Lock()
				w.closed = true
				w.mu.Unlock()

				err := w.Process()
				if err != nil {
					slog.Default().Error("Error during flush by context", "err", err)
					resultErr = err

					return
				}

				return
			case <-tck.C:
				slog.Debug("Flush by ticker")

				err := w.Process()
				if err != nil {
					slog.Default().Error("Error during flush by ticker", "err", err)
				}
			case <-w.counter:
				slog.Debug("Flush by counter")

				err := w.Process()
				if err != nil {
					slog.Default().Error("Error during default flush by ticker", "err", err)
				}
			}
		}
	})

	return resultErr
}
