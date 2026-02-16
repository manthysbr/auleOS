package services

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/manthysbr/auleOS/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

func TestJobScheduler_ConcurrencyLimit(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	config := SchedulerConfig{MaxConcurrentJobs: 2} // Limit to 2
	scheduler := NewJobScheduler(logger, config)

	ctx := context.Background()
	executeJob := func(ctx context.Context, job domain.Job) {
		// Start processing
	}
	scheduler.Start(ctx, executeJob)

	var runningJobs int32
	var maxRunningJobs int32
	var wg sync.WaitGroup

	totalJobs := 5
	wg.Add(totalJobs)

	// Mock execution that holds the slot for a bit
	mockExec := func(ctx context.Context, job domain.Job) {
		current := atomic.AddInt32(&runningJobs, 1)
		
		// Track peak concurrency
		for {
			max := atomic.LoadInt32(&maxRunningJobs)
			if current > max {
				if !atomic.CompareAndSwapInt32(&maxRunningJobs, max, current) {
					continue
				}
			}
			break
		}

		time.Sleep(100 * time.Millisecond) // Simulate work
		atomic.AddInt32(&runningJobs, -1)
		wg.Done()
	}

	// Override specific handler for test if possible?
	// The scheduler accepts the handler in Start() and it's private.
	// But `Start` sets `s.handler`.
	// Ideally we passed handler in New or Start. 
	// Wait, `Start` takes the handler. 
	// But I already called `Start` above with empty handler.
	// I need to restart or create new scheduler.
	
	scheduler2 := NewJobScheduler(logger, config)
	scheduler2.Start(ctx, mockExec)

	// Submit 5 jobs rapidly
	for i := 0; i < totalJobs; i++ {
		go scheduler2.SubmitJob(ctx, domain.Job{ID: domain.JobID("job")})
	}

	wg.Wait()

	// Assertions
	peak := atomic.LoadInt32(&maxRunningJobs)
	assert.LessOrEqual(t, peak, int32(2), "Should not exceed max concurrency")
	assert.Greater(t, peak, int32(0), "Should havrun some jobs")
}
