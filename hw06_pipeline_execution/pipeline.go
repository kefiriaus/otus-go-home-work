package hw06pipelineexecution

type (
	In  = <-chan interface{}
	Out = In
	Bi  = chan interface{}
)

type Stage func(in In) (out Out)

func headGuard(in In, done In) Out {
	out := make(Bi)
	go func() {
		defer close(out)
		for {
			select {
			case <-done:
				return
			case v, ok := <-in:
				if !ok {
					return
				}
				select {
				case out <- v:
				case <-done:
					return
				}
			}
		}
	}()
	return out
}

func tailGuard(in In, done In) Out {
	out := make(Bi)
	go func() {
		for {
			select {
			case <-done:
				close(out)
				//nolint:revive
				for range in {
				}
				return
			case v, ok := <-in:
				if !ok {
					close(out)
					return
				}
				select {
				case out <- v:
				case <-done:
					close(out)
					//nolint:revive
					for range in {
					}
					return
				}
			}
		}
	}()
	return out
}

func ExecutePipeline(in In, done In, stages ...Stage) Out {
	in = headGuard(in, done)
	for _, stage := range stages {
		in = stage(in)
	}
	return tailGuard(in, done)
}
