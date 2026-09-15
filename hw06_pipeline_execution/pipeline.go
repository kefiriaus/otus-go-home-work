package hw06pipelineexecution

type (
	In  = <-chan interface{}
	Out = In
	Bi  = chan interface{}
)

type Stage func(in In) (out Out)

func guard(in In, done In) Out {
	out := make(Bi)
	go func() {
		defer func() {
			close(out)
			//nolint:revive
			for range in {
			}
		}()

		for {
			select {
			case <-done:
				return
			default:
			}

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

func ExecutePipeline(in In, done In, stages ...Stage) Out {
	out := guard(in, done)
	for _, stage := range stages {
		out = guard(stage(out), done)
	}
	return out
}
