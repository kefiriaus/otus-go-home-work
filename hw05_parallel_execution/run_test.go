package hw05parallelexecution

import (
	"errors"
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak" //nolint:depguard
)

func TestRun(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("if were errors in first M tasks, than finished not more N+M tasks", func(t *testing.T) {
		tasksCount := 50
		tasks := make([]Task, 0, tasksCount)

		var runTasksCount int32

		for i := 0; i < tasksCount; i++ {
			err := fmt.Errorf("error from task %d", i)
			tasks = append(tasks, func() error {
				time.Sleep(time.Millisecond * time.Duration(rand.Intn(100)))
				atomic.AddInt32(&runTasksCount, 1)
				return err
			})
		}

		workersCount := 10
		maxErrorsCount := 23
		err := Run(tasks, workersCount, maxErrorsCount)

		require.Truef(t, errors.Is(err, ErrErrorsLimitExceeded), "actual err - %v", err)
		require.LessOrEqual(t, runTasksCount, int32(workersCount+maxErrorsCount), "extra tasks were started")
	})

	t.Run("tasks without errors", func(t *testing.T) {
		tasksCount := 50
		tasks := make([]Task, 0, tasksCount)

		var runTasksCount int32
		var sumTime time.Duration

		for i := 0; i < tasksCount; i++ {
			taskSleep := time.Millisecond * time.Duration(rand.Intn(100))
			sumTime += taskSleep

			tasks = append(tasks, func() error {
				time.Sleep(taskSleep)
				atomic.AddInt32(&runTasksCount, 1)
				return nil
			})
		}

		workersCount := 5
		maxErrorsCount := 1

		start := time.Now()
		err := Run(tasks, workersCount, maxErrorsCount)
		elapsedTime := time.Since(start)
		require.NoError(t, err)

		require.Equal(t, int32(tasksCount), runTasksCount, "not all tasks were completed")
		require.LessOrEqual(t, int64(elapsedTime), int64(sumTime/2), "tasks were run sequentially?")
	})
}

func TestRunConcurrencyWithoutSleep(t *testing.T) {
	defer goleak.VerifyNone(t)

	const (
		workersCount = 5
		tasksCount   = 20
	)

	release := make(chan struct{})
	var active, peak atomic.Int32

	tasks := make([]Task, tasksCount)
	for i := range tasks {
		tasks[i] = func() error {
			updatePeak(&peak, active.Add(1))
			<-release
			active.Add(-1)
			return nil
		}
	}

	errCh := make(chan error, 1)
	go func() { errCh <- Run(tasks, workersCount, 1) }()

	require.Eventually(t, func() bool {
		return active.Load() == workersCount
	}, time.Second, time.Millisecond)

	close(release)
	require.NoError(t, <-errCh)
	require.LessOrEqual(t, peak.Load(), int32(workersCount), "more than n tasks ran at once")
}

func updatePeak(peak *atomic.Int32, cur int32) {
	for {
		p := peak.Load()
		if cur <= p || peak.CompareAndSwap(p, cur) {
			return
		}
	}
}

func TestRunEdgeCases(t *testing.T) {
	defer goleak.VerifyNone(t)

	makeTasks := func(count int, isErr func(i int) bool, counter *atomic.Int32) []Task {
		tasks := make([]Task, count)
		for i := range tasks {
			fail := isErr(i)
			tasks[i] = func() error {
				counter.Add(1)
				if fail {
					return errors.New("task error")
				}
				return nil
			}
		}
		return tasks
	}
	never := func(int) bool { return false }
	always := func(int) bool { return true }
	even := func(i int) bool { return i%2 == 0 }

	t.Run("empty tasks", func(t *testing.T) {
		require.NoError(t, Run(nil, 5, 1))
	})

	t.Run("fewer tasks than workers", func(t *testing.T) {
		var cnt atomic.Int32
		require.NoError(t, Run(makeTasks(3, never, &cnt), 10, 1))
		require.Equal(t, int32(3), cnt.Load())
	})

	t.Run("errors below limit", func(t *testing.T) {
		var cnt atomic.Int32
		require.NoError(t, Run(makeTasks(10, even, &cnt), 3, 6))
		require.Equal(t, int32(10), cnt.Load())
	})

	t.Run("errors exactly at limit", func(t *testing.T) {
		var cnt atomic.Int32
		err := Run(makeTasks(10, even, &cnt), 3, 5)
		require.ErrorIs(t, err, ErrErrorsLimitExceeded)
	})

	t.Run("single worker, m=1", func(t *testing.T) {
		var cnt atomic.Int32
		err := Run(makeTasks(10, always, &cnt), 1, 1)
		require.ErrorIs(t, err, ErrErrorsLimitExceeded)
		require.LessOrEqual(t, cnt.Load(), int32(2))
	})
}
