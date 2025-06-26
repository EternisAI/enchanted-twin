package evolvingmemory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// DelayJob is a test job that simulates work with a time delay.
type DelayJob struct {
	ID    string
	Delay time.Duration
}

func (d DelayJob) Process(ctx context.Context) (string, error) {
	select {
	case <-time.After(d.Delay):
		return d.ID + " completed", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Simple test logger.
type testLogger struct {
	t *testing.T
}

func (l testLogger) Debugf(format string, args ...interface{}) {
	l.t.Logf("[DEBUG] "+format, args...)
}

func (l testLogger) Infof(format string, args ...interface{}) {
	l.t.Logf("[INFO] "+format, args...)
}

func TestWorkerPoolDynamicDistribution(t *testing.T) {
	// Create jobs: 1 slow (1s) + 9 fast (100ms)
	jobs := []DelayJob{
		{ID: "slow", Delay: 1000 * time.Millisecond},
		{ID: "fast1", Delay: 100 * time.Millisecond},
		{ID: "fast2", Delay: 100 * time.Millisecond},
		{ID: "fast3", Delay: 100 * time.Millisecond},
		{ID: "fast4", Delay: 100 * time.Millisecond},
		{ID: "fast5", Delay: 100 * time.Millisecond},
		{ID: "fast6", Delay: 100 * time.Millisecond},
		{ID: "fast7", Delay: 100 * time.Millisecond},
		{ID: "fast8", Delay: 100 * time.Millisecond},
		{ID: "fast9", Delay: 100 * time.Millisecond},
	}

	pool := NewWorkerPool[DelayJob](4, testLogger{t})

	ctx := context.Background()
	start := time.Now()

	results := pool.Process(ctx, jobs, 2*time.Second)

	// Collect all results
	resultCount := 0
	for range results {
		resultCount++
	}

	elapsed := time.Since(start)

	t.Logf("Total time: %v", elapsed)
	t.Logf("Results collected: %d", resultCount)

	// With dynamic distribution:
	// - One worker takes the slow job (1s)
	// - Other workers share the 9 fast jobs (100ms each)
	// - Total time should be ~1s (optimal)
	//
	// With static distribution:
	// - 10 jobs / 4 workers = 2.5 jobs per worker
	// - One worker would get slow + 2 fast = 1.2s

	assert.Less(t, elapsed, 1200*time.Millisecond, "Should be faster than static distribution")
	assert.GreaterOrEqual(t, elapsed, 1000*time.Millisecond, "Should take at least the slow job duration")
	assert.Equal(t, 10, resultCount, "Should process all jobs")
}
