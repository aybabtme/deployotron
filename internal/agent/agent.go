package agent

import (
	"fmt"
	"sync"
	"time"

	"github.com/aybabtme/deployotron/internal/container"
	"github.com/aybabtme/log"
)

// An Agent supervises programs.
type Agent struct {
	client container.Client

	mu        sync.Mutex
	instances map[container.ProgramID]map[container.ProcessID]*managedProcess
	started   map[container.ProcessID]*managedProcess
}

// New creates an agent that executes programs.
func New(client container.Client) *Agent {
	return &Agent{
		client:    client,
		instances: make(map[container.ProgramID]map[container.ProcessID]*managedProcess),
		started:   make(map[container.ProcessID]*managedProcess),
	}
}

// Start a program.
func (ag *Agent) Start(id container.ProgramID) error {
	prgm, err := ag.client.Programs().Pull(id)
	if err != nil {
		return fmt.Errorf("pulling program: %v", err)
	}
	ag.mu.Lock()
	defer ag.mu.Unlock()
	return ag.start(prgm)
}

func (ag *Agent) start(prgm container.Program) error {
	proc, err := ag.client.Processes().Create(prgm)
	if err != nil {
		return fmt.Errorf("creating process: %v", err)
	}
	if err := proc.Start(); err != nil {
		return fmt.Errorf("starting process: %v", err)
	}

	if _, ok := ag.started[proc.ID()]; ok {
		return fmt.Errorf("process is already managed: %v", proc.ID())
	}
	mproc := manage(proc)
	ag.recordInstance(mproc)
	return nil
}

// StopAll stops all processes of a program.
func (ag *Agent) StopAll(id container.ProgramID, timeout time.Duration) error {
	ag.mu.Lock()
	defer ag.mu.Unlock()
	for _, mproc := range ag.instances[id] {
		mproc.stop(timeout)
		ag.dropInstance(mproc)
	}
	return nil
}

// Restart all the process running a program while respecting a policy.
func (ag *Agent) Restart(policy RestartPolicy, id container.ProgramID) error {
	prgm, ok, err := ag.client.Programs().Get(id)
	switch {
	case err != nil:
		return fmt.Errorf("can't get program to restart: %v", err)
	case !ok:
		return fmt.Errorf("program %v isn't present", id)
	}
	ag.mu.Lock()
	defer ag.mu.Unlock()
	return ag.cycleProcesses(policy, prgm, prgm)
}

// Upgrade from a program to another while respecting the policy.
func (ag *Agent) Upgrade(policy RestartPolicy, from, to container.ProgramID) error {
	fromPrgm, ok, err := ag.client.Programs().Get(from)
	switch {
	case err != nil:
		return fmt.Errorf("can't get program to upgrade: %v", err)
	case !ok:
		return fmt.Errorf("program %v isn't present, thus cannot be upgraded", from)
	}

	toPrgm, err := ag.client.Programs().Pull(to)
	if err != nil {
		return fmt.Errorf("can't pull program to upgrade: %v", err)
	}

	ag.mu.Lock()
	defer ag.mu.Unlock()
	return ag.cycleProcesses(policy, fromPrgm, toPrgm)
}

func (ag *Agent) cycleProcesses(policy RestartPolicy, from, to container.Program) error {
	unordered, ok := ag.instances[from.ID()]
	if !ok {
		return fmt.Errorf("no instance of program %v is running", from)
	}
	count := len(unordered)
	ordered := make([]*managedProcess, 0, count)
	for _, inst := range unordered {
		ordered = append(ordered, inst)
	}

	stop := func(i int) error {
		proc := ordered[i]
		proc.stop(policy.Timeout())
		ag.dropInstance(proc)
		return nil
	}
	start := func(i int) error {
		if err := ag.start(to); err != nil {
			return fmt.Errorf("cycle loop failed to start after stop: %v", err)
		}
		return nil
	}

	return policy.Do(count, stop, start)
}

/*
 helpers
*/

func (ag *Agent) recordInstance(mproc *managedProcess) {
	prgmID := mproc.proc.Program().ID()
	procID := mproc.proc.ID()
	ag.started[procID] = mproc
	if _, ok := ag.instances[prgmID]; !ok {
		ag.instances[prgmID] = make(map[container.ProcessID]*managedProcess, 0)
	}
	ag.instances[prgmID][procID] = mproc
}

func (ag *Agent) dropInstance(mproc *managedProcess) {
	prgmID := mproc.proc.Program().ID()
	procID := mproc.proc.ID()
	instances, ok := ag.instances[prgmID]
	if !ok {
		panic(fmt.Sprintf("can't delete program %v from instances %#v", prgmID, ag.instances))
	}

	delete(ag.started, procID)
	delete(instances, procID)
	if len(instances) == 0 {
		delete(ag.instances, prgmID)
	}
}

func (ag *Agent) handleError(err error) {
	log.Err(err).Error("unexpected error")
}
