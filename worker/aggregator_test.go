package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestWaiter(t *testing.T) {
	const (
		totalItemsToAdd  = 23
		maxCountPerBatch = 10
	)

	// This sub-test demonstrates the failure when we don't wait.

	t.Run("Fails Without Close and Wait", func(t *testing.T) {
		var processedCount atomic.Uint32

		agg := NewAggregator[int](
			context.Background(),
			1,
			maxCountPerBatch, // Flush after 10 items
			10*time.Second,   // Use a long ticker to ensure flushing is by count
			func(items []int) error {
				processedCount.Add(uint32(len(items))) // nolint:gosec
				return nil
			},
		)
		go agg.Flusher()

		// 2. Action: Add 23 items.
		// This will trigger two flushes of 10 items each.
		// The final 3 items will remain in the buffer.
		for i := 0; i < totalItemsToAdd; i++ {
			agg.Add(i)
		}

		// 3. Problem: We don't wait for the final 3 items to be processed.
		// We give the background processor a moment to run, but there's no guarantee.
		time.Sleep(50 * time.Millisecond)

		// 4. Assertion: The test finishes before the background work is done.
		// We expect this assertion to FAIL because `processedCount` will be 20, not 23.
		// We use NotEqual to make the test pass, but demonstrate the logical error.
		finalCount := processedCount.Load()
		t.Logf("Items processed without waiting: %d", finalCount)
		require.NotEqual(t, uint32(totalItemsToAdd), finalCount, "The final batch was not processed")

		agg.Close() // Cleanup
	})

	// This sub-test demonstrates the success when we wait correctly.
	t.Run("Succeeds With Close and Wait", func(t *testing.T) {
		var processedCount atomic.Uint32

		agg := NewAggregator[int](
			context.Background(),
			1,
			maxCountPerBatch,
			10*time.Second,
			func(items []int) error {
				processedCount.Add(uint32(len(items))) // nolint:gosec
				return nil
			},
		)
		go agg.Flusher()

		// 2. Action: Add the same 23 items.
		for i := 0; i < totalItemsToAdd; i++ {
			agg.Add(i)
		}

		// 3. Solution: Use the correct synchronization pattern.
		agg.Close() // Signal the Flusher to process any remaining items and shut down.
		agg.Wait()  // Pause this test until the aggregator's count is zero.

		// 4. Assertion: The test waits until all background work is complete.
		// This assertion will reliably pass.
		finalCount := processedCount.Load()
		t.Logf("Items processed after waiting: %d", finalCount)
		require.Equal(t, uint32(totalItemsToAdd), finalCount, "All items should be processed")
	})
}

func TestAggregator(t *testing.T) {
	t.Run("add values to aggregator", func(t *testing.T) {
		a := NewAggregator(context.TODO(), 1, 100, time.Second, func(_ []any) error {
			return nil
		})
		primID := primitive.NewObjectID()
		a.Add(primID)
		require.Equal(t, a.Count(), 1)
	})

	t.Run("add to aggregator multiple times", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())

		a := NewAggregator(ctx, 1, 100, time.Second, func(_ []any) error {
			return nil
		})
		primID := primitive.NewObjectID()
		a.Add(primID)
		require.Equal(t, a.Count(), 1)

		a.Add(primID)
		require.Equal(t, a.Count(), 2)

		go func() {
			time.Sleep(100 * time.Millisecond)
			cancel()
		}()

		err := a.Flusher()
		require.NoError(t, err)
		require.Equal(t, a.Count(), 0)
	})

	t.Run("test lock", func(t *testing.T) {
		a := NewAggregator(context.TODO(), 1, 3, time.Second, func(_ []any) error {
			return nil
		})
		primID := primitive.NewObjectID()

		var counter int32

		go func() {
			for i := 0; i < 4; i++ {
				atomic.AddInt32(&counter, 1)
				a.Add(primID)
			}
		}()

		time.Sleep(time.Millisecond * 10)

		if atomic.LoadInt32(&counter) == 3 {
			err := a.Process()
			require.NoError(t, err)
		}
	})

	t.Run("error processing", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.TODO())

		expectedErr := errors.New("mock error")
		a := NewAggregator(ctx, 2, 100, time.Second, func(_ []any) error {
			return expectedErr
		})

		primID := primitive.NewObjectID()
		a.Add(primID)
		require.Equal(t, a.Count(), 1)

		cancel()

		err := a.Flusher()
		require.Equal(t, expectedErr, err)
	})
}
