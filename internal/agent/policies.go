package agent

import (
	"fmt"
	"sync"
	"time"
)

// RestartPolicy tells the agent how to restart or upgrade instances of a
// running program.
type RestartPolicy interface {
	Do(count int, stop func(int) error, start func(int) error) error
	Timeout() time.Duration
}

type restarter struct {
	timeout time.Duration
	do      func(int, func(int) error, func(int) error) error
}

func (policy restarter) Timeout() time.Duration { return policy.timeout }
func (policy restarter) Do(count int, stop func(int) error, start func(int) error) error {
	return policy.do(count, stop, start)
}

// PolicyStartBeforeStop will start the next process before stopping the
// current one. By default, processes are stopped before being started again.
func PolicyStartBeforeStop(policy RestartPolicy) RestartPolicy {
	return &restarter{
		timeout: policy.Timeout(),
		do: func(count int, stop func(int) error, start func(int) error) error {
			start, stop = stop, start // flip them around
			return policy.Do(count, stop, start)
		},
	}
}

// PolicyStopTimeout adds a timeout to the stop call on process.
func PolicyStopTimeout(policy RestartPolicy, timeout time.Duration) RestartPolicy {
	return &restarter{
		timeout: timeout,
		do:      policy.Do,
	}
}

// PolicyAllAtOnce restarts everything at once.
func PolicyAllAtOnce() RestartPolicy {
	return &restarter{
		do: func(count int, stop func(int) error, start func(int) error) error {

			wg := sync.WaitGroup{}
			errc := make(chan error, 1)
			for i := 0; i < count; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					if err := stop(i); err != nil {
						select {
						case errc <- fmt.Errorf("one-shot restart, stopping process %d: %v", i, err):
						default:
						}
						return
					}
					if err := start(i); err != nil {
						select {
						case errc <- fmt.Errorf("one-shot restart, starting process %d: %v", i, err):
						default:
						}
						return
					}
				}(i)
			}
			wg.Wait()
			close(errc)
			return <-errc
		},
	}
}

// PolicyRolling restarts one process at a time.
func PolicyRolling() RestartPolicy {
	return &restarter{
		do: func(count int, stop func(int) error, start func(int) error) error {
			for i := 0; i < count; i++ {

				if err := stop(i); err != nil {
					return fmt.Errorf("rolling restart, stopping process %d: %v", i, err)
				}
				if err := start(i); err != nil {
					return fmt.Errorf("rolling restart, starting process %d: %v", i, err)
				}

			}
			return nil
		},
	}
}
