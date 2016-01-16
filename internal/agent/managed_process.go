package agent

import (
	"fmt"
	"time"

	"github.com/aybabtme/deployotron/internal/container"
	"github.com/aybabtme/log"
)

type managedProcess struct {
	kill chan time.Duration
	done chan struct{}
	ag   *agent
	proc container.Process
}

func manage(proc container.Process) *managedProcess {
	kill := make(chan time.Duration, 1)
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
	timeout := <-mproc.kill
	close(mproc.done) // tell the keepAlive loop to give up
	if timeout != 0 {
		stopped := make(chan struct{}, 0)
		go func() {
			if err := mproc.proc.Stop(); err != nil {
				mproc.ag.handleError(err)
			} else {
				close(stopped)
			}
		}()
		select {
		case <-time.After(timeout):
		case <-stopped:
			return
		}
		if err := mproc.proc.Kill(); err != nil {
			mproc.ag.handleError(fmt.Errorf("killing process %v: %v", mproc.proc.ID(), err))
		}
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
			}
		}
	}
}

func (mproc *managedProcess) stop(timeout time.Duration) {
	select {
	case mproc.kill <- timeout:
	default:
	}
}
