package hw06pipelineexecution

type (
	In  = <-chan interface{}
	Out = In
	Bi  = chan interface{}
)

type Stage func(in In) (out Out)

func WrapStage(in In, done In) Out {
	out := make(Bi)
	go func() {
		defer close(out)
		for {
			select {
			case v, ok := <-in:
				if !ok {
					return
				}
				select {
				case out <- v:
				case <-done:
					return
				}
			case <-done:
				return
			}
		}
	}()
	return out
}

func ExecuteStage(in In, done In, stage Stage) Out {
	out := make(Bi)
	stageOut := stage(WrapStage(in, done))
	go func() {
		defer close(out)
		for v := range stageOut {
			select {
			case out <- v:
			case <-done:
				for range stageOut {
				}
				return
			}
		}
	}()
	return out
}

func ExecutePipeline(in In, done In, stages ...Stage) Out {
	if len(stages) == 0 {
		return in
	}

	for _, stage := range stages {
		in = ExecuteStage(in, done, stage)
	}
	return in
}
