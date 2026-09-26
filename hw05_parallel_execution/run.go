package hw05parallelexecution

import (
	"errors"
	"sync"
	"sync/atomic"
)

var ErrErrorsLimitExceeded = errors.New("errors limit exceeded")

type Task func() error

// Run starts tasks in n goroutines and stops its work when receiving m errors from tasks.
func Run(tasks []Task, n, m int) error {
	if len(tasks) == 0 {
		return nil
	}
	workers := min(max(n, 1), len(tasks))

	ignoreErrors := m <= 0
	limit := int64(m)
	var errCount atomic.Int64
	exceeded := func() bool {
		return !ignoreErrors && errCount.Load() >= limit
	}

	taskCh := make(chan Task)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for task := range taskCh {
				if err := task(); err != nil {
					errCount.Add(1)
				}
			}
		}()
	}

	for _, task := range tasks {
		if exceeded() {
			break
		}
		taskCh <- task
	}
	close(taskCh)
	wg.Wait()

	if exceeded() {
		return ErrErrorsLimitExceeded
	}
	return nil
}
