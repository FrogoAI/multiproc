package buffer

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FrogoAI/testutils"
)

func TestDelayExecute(t *testing.T) {
	ctx := context.TODO()

	tests := []struct {
		name    string
		delay   time.Duration
		addErr  error
		waitErr error
	}{
		{
			name:  "success delay",
			delay: time.Millisecond,
		},
		{
			name:  "success no delay",
			delay: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDelay(ctx, tt.delay)

			var num int64

			testutils.Equal(t, d.Add(func() error {
				atomic.AddInt64(&num, 1)
				return nil
			}), tt.addErr)
			testutils.Equal(t, d.Wait(), tt.waitErr)
			testutils.Equal(t, atomic.LoadInt64(&num), int64(1))
		})
	}
}
