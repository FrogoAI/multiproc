# multiproc

> A collection of high-performance, generic concurrency primitives for Go.

`multiproc` provides robust building blocks for building scalable, event-driven Go applications. It fills the gap between raw goroutines and complex orchestration frameworks by offering type-safe worker pools, smart channel aggregators, delayed execution buffers, and enhanced synchronization primitives.

## Features

### 1. Smart Aggregator (`worker.Aggregator`)

Efficiently groups individual records into batches for bulk processing (e.g., database inserts).

* **Dual Triggers:** Flushes batches when they reach a specific `size` OR when a `ticker` interval elapses.
* **Concurrency:** Supports multiple flush routines for high-throughput ingestion.
* **Generic:** Type-safe implementation using Go generics `[K any]`.

### 2. Advanced Worker Pool (`worker.WorkersPool`)

A generic worker pool designed for dynamic workloads.

* **Persistent Workers:** Standard workers that process a channel until closed.
* **Temporal Workers:** Smart workers that process messages but shut down automatically after an `idleTimeout`. Ideal for handling burst traffic without permanently consuming resources.
* **Observability:** Tracks queue size and active worker status.

### 3. Resettable Once (`sync.Once`)

An enhanced version of the standard `sync.Once`.

* **Retry on Error:** If the action returns an error, it is *not* marked as done, allowing retries.
* **Reset:** Manually reset the state to allow the action to run again.

### 4. Delayed Execution (`buffer.Delay`)

* **Async Delay:** Schedules functions to run after a specified duration using a background worker pool.

## Installation

```bash
go get github.com/FrogoAI/multiproc

```

## Usage Examples

### Batch Aggregator

Combine a stream of individual items into batches of 100 or every 1 second—whichever comes first.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/FrogoAI/multiproc/worker"
)

func main() {
	ctx := context.Background()

	// Processor function: handles the batched slice
	processBatch := func(items []string) error {
		fmt.Printf("Processing batch of %d items: %v\n", len(items), items)
		return nil
	}

	// Create Aggregator: 1 flushing goroutine, Batch Size 100, Timeout 1s
	ag := worker.NewAggregator[string](ctx, 1, 100, 1*time.Second, processBatch)

	// Start the flusher in the background
	go func() {
		if err := ag.Flusher(); err != nil {
			fmt.Printf("Flusher error: %v\n", err)
		}
	}()

	// Add items
	for i := 0; i < 150; i++ {
		ag.Add(fmt.Sprintf("msg-%d", i))
	}

	// Wait for processing to drain
	ag.Wait()
}

```

### Dynamic Worker Pool (Temporal)

Handle burst traffic by spawning workers that die off when idle.

```go
package main

import (
	"context"
	"time"

	"github.com/FrogoAI/multiproc/worker"
)

func main() {
	ctx := context.Background()
	pool := worker.NewWorkersPool[string](ctx)
	jobs := make(chan string)

	// Handler logic
	handler := func(ctx context.Context, msg string) error {
		// Process message...
		return nil
	}

	// Spawn a temporal worker
	// It will process messages from 'jobs'
	// It will exit if 'jobs' has no new messages for 5 seconds
	pool.Execute(func(ctx context.Context) error {
		return pool.TemporalWorker(
			ctx, 
			5*time.Second, // Idle Timeout
			nil,           // Optional callback on timeout
			jobs,          // Input channel
			handler,       // Processing function
		)
	})
	
	jobs <- "payload"
	
	pool.Wait()
}

```

### Resettable Sync Once

Execute initialization logic that allows retries if it fails.

```go
package main

import (
	"errors"
	"fmt"
	
	"github.com/FrogoAI/multiproc/sync"
)

func main() {
	var once sync.Once
	count := 0

	action := func() error {
		count++
		if count < 2 {
			return errors.New("network error")
		}
		fmt.Println("Success!")
		return nil
	}

	// First attempt fails -> Error returned, NOT marked done
	err := once.Do(action) 
	// err != nil
	
	// Second attempt succeeds -> Marked done
	err = once.Do(action)
	// Success!
	
	// Third attempt -> Skipped (already done)
	once.Do(action)
}

```

## License

[MIT](https://www.google.com/search?q=LICENSE)