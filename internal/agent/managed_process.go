package agent

import (
	"fmt"
	"time"

	"github.com/aybabtme/deployotron/internal/container"
	"github.com/aybabtme/log"
)

type managedProcess struct {
	kill chan *stopJob
	done chan struct{}
	ag   *Agent
	proc container.Process
}

func manage(proc container.Process) *managedProcess {
	kill := make(chan *stopJob, 1)
	done := make(chan struct{})
	mproc := &managedProcess{kill: kill, proc: proc, done: done}
	go mproc.listenStop()
	go mproc.keepAlive()
	return mproc
}

func (mproc *managedProcess) handleError(err error) {
	log.KV("proc.id", mproc.proc.ID()).Err(err).Error("unexpected error")
}

func (mproc *managedProcess) listenStop() {
	job := <-mproc.kill
	defer close(job.done)
	close(mproc.done) // tell the keepAlive loop to give up
	if job.timeout != 0 {
		stopped := make(chan struct{}, 0)
		go func() {
			if err := mproc.proc.Stop(job.timeout); err != nil {
				mproc.ag.handleError(err)
			} else {
				close(stopped)
			}
		}()
		// give it a chance to stop cleanly
		select {
		case <-time.After(job.timeout):
		case <-stopped:
			return
		}
	}

	if err := mproc.proc.Kill(); err != nil {
		mproc.ag.handleError(fmt.Errorf("killing process %v: %v", mproc.proc.ID(), err))
	}
}

func (mproc *managedProcess) keepAlive() {
	// TODO(antoine): accept a policy for restarts, like:
	//   - forever/limited attempts to restart
	//   - immediate/backoff
	proc := mproc.proc
	for {
		err := proc.Wait()
		select {
		case <-mproc.done:
			return // expected to die
		default:
		}
		if err != nil {
			mproc.handleError(fmt.Errorf("waiting for process %v: %v", proc.ID(), err))
		}

		// restart it
		for serr := proc.Start(); serr != nil; serr = proc.Start() {
			select {
			case <-mproc.done:
				return // expected to die
			default:
				mproc.handleError(fmt.Errorf("trying to restart process %v: %v", proc.ID(), serr))
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
}

type stopJob struct {
	timeout time.Duration
	done    chan struct{}
}

func (mproc *managedProcess) stop(timeout time.Duration) {
	job := &stopJob{timeout: timeout, done: make(chan struct{})}
	select {
	case mproc.kill <- job:
		<-job.done
	default:
	}
}
