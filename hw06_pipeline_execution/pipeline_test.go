package hw06pipelineexecution

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

const (
	sleepPerStage = time.Millisecond * 100
	fault         = sleepPerStage / 2
)

func TestPipeline(t *testing.T) {
	// Stage generator
	g := func(_ string, f func(v interface{}) interface{}) Stage {
		return func(in In) Out {
			out := make(Bi)
			go func() {
				defer close(out)
				for v := range in {
					time.Sleep(sleepPerStage)
					out <- f(v)
				}
			}()
			return out
		}
	}

	stages := []Stage{
		g("Dummy", func(v interface{}) interface{} { return v }),
		g("Multiplier (* 2)", func(v interface{}) interface{} { return v.(int) * 2 }),
		g("Adder (+ 100)", func(v interface{}) interface{} { return v.(int) + 100 }),
		g("Stringifier", func(v interface{}) interface{} { return strconv.Itoa(v.(int)) }),
	}

	t.Run("simple case", func(t *testing.T) {
		in := make(Bi)
		data := []int{1, 2, 3, 4, 5}

		go func() {
			for _, v := range data {
				in <- v
			}
			close(in)
		}()

		result := make([]string, 0, 10)
		start := time.Now()
		for s := range ExecutePipeline(in, nil, stages...) {
			result = append(result, s.(string))
		}
		elapsed := time.Since(start)

		require.Equal(t, []string{"102", "104", "106", "108", "110"}, result)
		require.Less(t,
			int64(elapsed),
			// ~0.8s for processing 5 values in 4 stages (100ms every) concurrently
			int64(sleepPerStage)*int64(len(stages)+len(data)-1)+int64(fault))
	})

	t.Run("done case", func(t *testing.T) {
		in := make(Bi)
		done := make(Bi)
		data := []int{1, 2, 3, 4, 5}

		// Abort after 200ms
		abortDur := sleepPerStage * 2
		go func() {
			<-time.After(abortDur)
			close(done)
		}()

		go func() {
			for _, v := range data {
				in <- v
			}
			close(in)
		}()

		result := make([]string, 0, 10)
		start := time.Now()
		for s := range ExecutePipeline(in, done, stages...) {
			result = append(result, s.(string))
		}
		elapsed := time.Since(start)

		require.Len(t, result, 0)
		require.Less(t, int64(elapsed), int64(abortDur)+int64(fault))
	})
}

func TestAllStageStop(t *testing.T) {
	wg := sync.WaitGroup{}
	// Stage generator
	g := func(_ string, f func(v interface{}) interface{}) Stage {
		return func(in In) Out {
			out := make(Bi)
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer close(out)
				for v := range in {
					time.Sleep(sleepPerStage)
					out <- f(v)
				}
			}()
			return out
		}
	}

	stages := []Stage{
		g("Dummy", func(v interface{}) interface{} { return v }),
		g("Multiplier (* 2)", func(v interface{}) interface{} { return v.(int) * 2 }),
		g("Adder (+ 100)", func(v interface{}) interface{} { return v.(int) + 100 }),
		g("Stringifier", func(v interface{}) interface{} { return strconv.Itoa(v.(int)) }),
	}

	t.Run("done case", func(t *testing.T) {
		in := make(Bi)
		done := make(Bi)
		data := []int{1, 2, 3, 4, 5}

		// Abort after 200ms
		abortDur := sleepPerStage * 2
		go func() {
			<-time.After(abortDur)
			close(done)
		}()

		go func() {
			for _, v := range data {
				in <- v
			}
			close(in)
		}()

		result := make([]string, 0, 10)
		for s := range ExecutePipeline(in, done, stages...) {
			result = append(result, s.(string))
		}
		wg.Wait()

		require.Len(t, result, 0)

	})
}

func TestNoGoroutineLeakOnDone(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	g := func(f func(v interface{}) interface{}) Stage {
		return func(in In) Out {
			out := make(Bi)
			go func() {
				defer close(out)
				for v := range in {
					time.Sleep(sleepPerStage)
					out <- f(v)
				}
			}()
			return out
		}
	}
	stages := []Stage{
		g(func(v interface{}) interface{} { return v }),
		g(func(v interface{}) interface{} { return v.(int) * 2 }),
		g(func(v interface{}) interface{} { return v.(int) + 100 }),
	}

	in := make(Bi)
	done := make(Bi)

	go func() {
		defer close(in)
		for i := 0; i < 1000; i++ {
			select {
			case in <- i:
			case <-done:
				return
			}
		}
	}()

	go func() {
		<-time.After(sleepPerStage * 2)
		close(done)
	}()

	//nolint:revive
	for range ExecutePipeline(in, done, stages...) {
	}
}

func TestPipelineEdgeCases(t *testing.T) {
	t.Run("no stages", func(t *testing.T) {
		in := make(Bi)
		go func() {
			defer close(in)
			in <- 1
			in <- 2
		}()

		result := make([]interface{}, 0, 2)
		for v := range ExecutePipeline(in, nil) {
			result = append(result, v)
		}
		require.Equal(t, []interface{}{1, 2}, result)
	})

	t.Run("empty input", func(t *testing.T) {
		in := make(Bi)
		close(in)

		stage := func(in In) Out {
			out := make(Bi)
			go func() {
				defer close(out)
				for v := range in {
					out <- v
				}
			}()
			return out
		}

		result := make([]interface{}, 0)
		for v := range ExecutePipeline(in, nil, stage, stage) {
			result = append(result, v)
		}
		require.Empty(t, result)
	})

	t.Run("done closed before start", func(t *testing.T) {
		in := make(Bi)
		done := make(Bi)
		close(done)

		go func() {
			defer close(in)
			for i := 0; i < 5; i++ {
				select {
				case in <- i:
				case <-done:
					return
				}
			}
		}()

		stage := func(in In) Out {
			out := make(Bi)
			go func() {
				defer close(out)
				for v := range in {
					time.Sleep(sleepPerStage)
					out <- v
				}
			}()
			return out
		}

		start := time.Now()
		result := make([]interface{}, 0)
		for v := range ExecutePipeline(in, done, stage, stage) {
			result = append(result, v)
		}
		require.Empty(t, result)
		require.Less(t, int64(time.Since(start)), int64(fault))
	})
}
