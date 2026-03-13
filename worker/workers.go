package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/FrogoAI/memory/utils"
)

type Worker struct {
	id      string
	ctx     context.Context
	cancel  context.CancelFunc
	fn      func(ctx context.Context) error
	stopped atomic.Bool
}

func NewWorker(ctx context.Context, fn func(ctx context.Context) error) *Worker {
	ctx, cancel := context.WithCancel(ctx)

	return &Worker{
		id:     uuid.NewString(),
		ctx:    ctx,
		cancel: cancel,
		fn:     fn,
	}
}

func (w *Worker) ID() string {
	return w.id
}

func (w *Worker) IsStopped() bool {
	return w.stopped.Load()
}

func (w *Worker) Context() context.Context {
	return w.ctx
}

func (w *Worker) Cancel() {
	w.cancel()
	w.stopped.Store(true)
}

func (w *Worker) Handle() error {
	defer w.stopped.Store(true)

	if w.fn == nil {
		return nil
	}

	return w.fn(w.ctx)
}

type WorkersPool[K any] struct {
	ctx    context.Context
	cancel func()

	wg   sync.WaitGroup
	mu   sync.Mutex
	errs []error

	queue uint64
	done  uint64

	workers *utils.SafeMap[string, *Worker]
	once    sync.Once
}

func NewWorkersPool[K any](
	ctx context.Context,
) *WorkersPool[K] {
	ctx, cancel := context.WithCancel(ctx)

	return &WorkersPool[K]{
		ctx:     ctx,
		cancel:  cancel,
		workers: utils.NewSafeMap[string, *Worker](nil),
	}
}

func (w *WorkersPool[K]) Execute(fn func(ctx context.Context) error) {
	worker := NewWorker(w.ctx, fn)
	w.workers.Set(worker.ID(), worker)

	w.wg.Add(1)

	atomic.AddUint64(&w.queue, 1)

	go func() {
		defer atomic.AddUint64(&w.done, 1)
		defer w.wg.Done()

		if err := worker.Handle(); err != nil {
			w.accumulateErr(err)
			w.Stop()
		}
	}()
}

func (w *WorkersPool[K]) accumulateErr(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.errs = append(w.errs, err)
}

func (w *WorkersPool[K]) Size() uint64 {
	totalAdd := atomic.LoadUint64(&w.queue)
	totalDone := atomic.LoadUint64(&w.done)

	return totalAdd - totalDone
}

func (w *WorkersPool[K]) Stop() {
	w.once.Do(func() {
		for _, worker := range w.workers.GetMap() {
			w.Remove(worker.ID())
			worker.Cancel()
		}

		w.cancel()
	})
}

func (w *WorkersPool[K]) Get(id string) (*Worker, bool) {
	return w.workers.Get(id)
}

func (w *WorkersPool[K]) Remove(id string) {
	w.workers.Remove(id)
}

func (w *WorkersPool[K]) Wait() error {
	w.wg.Wait()
	defer w.cancel()

	w.mu.Lock()
	defer w.mu.Unlock()
	return errors.Join(w.errs...)
}

func (w *WorkersPool[K]) TemporalWorker(
	ctx context.Context,
	idleTimeout time.Duration,
	timeout func(),
	ch chan K,
	handler func(context.Context, K) error,
) error {
	timer := time.NewTimer(idleTimeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			if timeout != nil {
				timeout()
			}

			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}

			if handler != nil {
				err := handler(ctx, msg)
				if err != nil {
					slog.Debug("error during process message", "err", err)
				}
			}

			timer.Reset(idleTimeout)
		}
	}
}

func (w *WorkersPool[K]) PersistentWorker(
	ctx context.Context,
	ch chan K,
	handler func(context.Context, K) error,
) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}

			if handler != nil {
				err := handler(ctx, msg)
				if err != nil {
					slog.Error("error during process message", "err", err)
				}
			}
		}
	}
}
