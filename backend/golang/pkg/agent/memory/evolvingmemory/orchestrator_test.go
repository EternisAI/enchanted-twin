package evolvingmemory

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// BenchmarkDynamicWorkers demonstrates the dynamic worker pattern with simulated processing delays.
func BenchmarkDynamicWorkers(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	// Test delays in milliseconds: one slow job + many fast jobs
	delays := []int{1000, 100, 100, 100, 100, 100, 100, 100, 100, 100}

	// processJobsWithWorkers simulates the dynamic worker pattern
	processJobsWithWorkers := func(delays []int, workerCount int) (time.Duration, map[int]int) {
		jobQueue := make(chan int, len(delays))
		for _, delay := range delays {
			jobQueue <- delay
		}
		close(jobQueue)

		workerJobCounts := make(map[int]int)
		var mu sync.Mutex
		var wg sync.WaitGroup

		start := time.Now()

		// Start workers
		for i := 0; i < workerCount; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				count := 0
				for delayMs := range jobQueue {
					// Simulate processing with the delay
					time.Sleep(time.Duration(delayMs) * time.Millisecond)
					count++
				}
				mu.Lock()
				workerJobCounts[workerID] = count
				mu.Unlock()
			}(i)
		}

		wg.Wait()
		return time.Since(start), workerJobCounts
	}

	// Test with different worker counts
	workerCounts := []int{1, 2, 4}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("Workers-%d", workers), func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				elapsed, workerDistribution := processJobsWithWorkers(delays, workers)

				// Calculate total expected time for sequential processing
				var totalSequentialTime time.Duration
				for _, delayMs := range delays {
					totalSequentialTime += time.Duration(delayMs) * time.Millisecond
				}

				speedup := float64(totalSequentialTime) / float64(elapsed)
				efficiency := speedup / float64(workers) * 100

				b.Logf("Workers: %d, Jobs: %d, Time: %v, Speedup: %.2fx, Efficiency: %.1f%%",
					workers, len(delays), elapsed, speedup, efficiency)

				// Log work distribution
				for workerID, count := range workerDistribution {
					b.Logf("  Worker %d processed %d jobs (%.1f%%)",
						workerID, count, float64(count)/float64(len(delays))*100)
				}
			}
		})
	}
}

// TestWorkerDistribution demonstrates the advantage of dynamic work distribution.
func TestWorkerDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping distribution test in short mode")
	}

	// Simulate static distribution (pre-assigned work)
	simulateStaticDistribution := func(delays []int, workers int) time.Duration {
		// Divide jobs evenly among workers
		jobsPerWorker := len(delays) / workers
		remainder := len(delays) % workers

		var wg sync.WaitGroup
		start := time.Now()

		for i := 0; i < workers; i++ {
			wg.Add(1)
			startIdx := i * jobsPerWorker
			endIdx := (i + 1) * jobsPerWorker
			if i == workers-1 {
				endIdx += remainder
			}

			go func(workerDelays []int, workerID int) {
				defer wg.Done()
				for _, delayMs := range workerDelays {
					time.Sleep(time.Duration(delayMs) * time.Millisecond)
				}
			}(delays[startIdx:endIdx], i)
		}

		wg.Wait()
		return time.Since(start)
	}

	// Simulate dynamic distribution (work-stealing)
	simulateDynamicDistribution := func(delays []int, workers int) time.Duration {
		jobQueue := make(chan int, len(delays))
		for _, delay := range delays {
			jobQueue <- delay
		}
		close(jobQueue)

		var wg sync.WaitGroup
		start := time.Now()

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for delayMs := range jobQueue {
					time.Sleep(time.Duration(delayMs) * time.Millisecond)
				}
			}(i)
		}

		wg.Wait()
		return time.Since(start)
	}

	// Delays in milliseconds: 1 slow job + 9 fast jobs
	delays := []int{1000, 100, 100, 100, 100, 100, 100, 100, 100, 100}

	workers := 4

	staticTime := simulateStaticDistribution(delays, workers)
	dynamicTime := simulateDynamicDistribution(delays, workers)

	t.Logf("Jobs: 1 slow (1000ms) + 9 fast (100ms each)")
	t.Logf("Workers: %d", workers)
	t.Logf("Static distribution time: %v", staticTime)
	t.Logf("Dynamic distribution time: %v", dynamicTime)
	t.Logf("Dynamic is %.1fx faster", float64(staticTime)/float64(dynamicTime))

	// In static distribution with 4 workers:
	// Worker 0: slow job (1000ms) + 2 fast jobs (200ms) = 1200ms
	// Worker 1: 3 fast jobs = 300ms
	// Worker 2: 2 fast jobs = 200ms
	// Worker 3: 2 fast jobs = 200ms
	// Total: 1200ms (bottlenecked by worker 0)

	// In dynamic distribution:
	// Worker 0 takes the slow job (1000ms)
	// Workers 1-3 process all 9 fast jobs in parallel (300ms each)
	// Total: 1000ms
}
