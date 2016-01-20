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

/*
 General API
*/

// ListAll returns all programs and their currently instanticated processes.
func (ag *Agent) ListAll() map[container.ProgramID][]container.ProcessID {
	ag.mu.Lock()
	defer ag.mu.Unlock()
	out := make(map[container.ProgramID][]container.ProcessID, len(ag.instances))
	for prgmID, mprocs := range ag.instances {
		procs := make([]container.ProcessID, 0, len(mprocs))
		for _, mproc := range mprocs {
			procs = append(procs, mproc.proc.ID())
		}
		out[prgmID] = procs
	}
	return out
}

// RestartAll restart all programs and their currently instanticated processes.
func (ag *Agent) RestartAll(policy RestartPolicy) error {
	ag.mu.Lock()
	defer ag.mu.Unlock()

	for _, procs := range ag.instances {

		// hack
		var prgmID container.ProgramID
		for _, proc := range procs {
			prgmID = proc.proc.Program().ID()
			break
		}

		prgm, ok, err := ag.client.Programs().Get(prgmID)
		if !ok {
			panic(fmt.Sprintf("program %v should be present, internal structure is inconsistent: %#v", prgmID, ag.instances))
		}
		if err != nil {
			return fmt.Errorf("restarting all processes, retrieving program %v: %v", prgmID, err)
		}
		if err := ag.cycleProcesses(policy, prgm, prgm); err != nil {
			return fmt.Errorf("restarting all processes, cycling program %v: %v", prgmID, err)
		}
	}
	return nil
}

/*
 Process scoped API
*/

// StartProcess a process running the given program.
func (ag *Agent) StartProcess(id container.ProgramID) (container.ProcessID, error) {
	prgm, err := ag.client.Programs().Pull(id)
	if err != nil {
		return "", fmt.Errorf("pulling program: %v", err)
	}
	ag.mu.Lock()
	defer ag.mu.Unlock()
	return ag.startProcess(prgm)
}

func (ag *Agent) startProcess(prgm container.Program) (container.ProcessID, error) {
	proc, err := ag.client.Processes().Create(prgm)
	if err != nil {
		return "", fmt.Errorf("creating process: %v", err)
	}
	if err := proc.Start(); err != nil {
		return "", fmt.Errorf("starting process: %v", err)
	}

	if _, ok := ag.started[proc.ID()]; ok {
		return "", fmt.Errorf("process is already managed: %v", proc.ID())
	}
	mproc := manage(proc)
	ag.recordInstance(mproc)
	return proc.ID(), nil
}

// StopProcess stops a running process.
func (ag *Agent) StopProcess(id container.ProcessID, timeout time.Duration) error {
	ag.mu.Lock()
	defer ag.mu.Unlock()
	mproc, ok := ag.started[id]
	if !ok {
		return fmt.Errorf("no such process: %#v", id)
	}
	mproc.stop(timeout)
	ag.dropInstance(mproc)
	return nil
}

// RestartProcess restarts a single process.
func (ag *Agent) RestartProcess(policy RestartPolicy, id container.ProcessID) error {
	ag.mu.Lock()
	defer ag.mu.Unlock()
	mproc, ok := ag.started[id]
	if !ok {
		return fmt.Errorf("no such process")
	}

	stop := func(i int) error {
		mproc.stop(policy.Timeout())
		ag.dropInstance(mproc)
		return nil
	}
	start := func(i int) error {
		if _, err := ag.startProcess(mproc.proc.Program()); err != nil {
			return fmt.Errorf("restart failed to start: %v", err)
		}
		return nil
	}

	return policy.Do(1, stop, start)
}

// UpgradeProcess upgrades a single process to a new program.
func (ag *Agent) UpgradeProcess(policy RestartPolicy, id container.ProcessID, to container.ProgramID) error {
	toPrgm, err := ag.client.Programs().Pull(to)
	if err != nil {
		return fmt.Errorf("can't pull program to upgrade: %v", err)
	}

	// we pull programs before locking
	ag.mu.Lock()
	defer ag.mu.Unlock()
	mproc, ok := ag.started[id]
	if !ok {
		return fmt.Errorf("no such process")
	}

	stop := func(i int) error {
		mproc.stop(policy.Timeout())
		ag.dropInstance(mproc)
		return nil
	}
	start := func(i int) error {
		if _, err := ag.startProcess(toPrgm); err != nil {
			return fmt.Errorf("upgrade failed to start: %v", err)
		}
		return nil
	}

	return policy.Do(1, stop, start)
}

/*
 Program scoped API
*/

// ListProgram returns all the running instances of a program.
func (ag *Agent) ListProgram(id container.ProgramID) ([]container.ProcessID, error) {
	ag.mu.Lock()
	defer ag.mu.Unlock()
	var procIDs []container.ProcessID
	for _, proc := range ag.instances[id] {
		procIDs = append(procIDs, proc.proc.ID())
	}
	return procIDs, nil
}

// StopProgram stops all processes of a program.
func (ag *Agent) StopProgram(id container.ProgramID, timeout time.Duration) error {
	ag.mu.Lock()
	defer ag.mu.Unlock()
	for _, mproc := range ag.instances[id] {
		mproc.stop(timeout)
		ag.dropInstance(mproc)
	}
	return nil
}

// RestartProgram the processes running a program while respecting a policy.
func (ag *Agent) RestartProgram(policy RestartPolicy, id container.ProgramID) error {
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

// UpgradeProgram upgrades all instances of a program to another program
// while respecting the policy.
func (ag *Agent) UpgradeProgram(policy RestartPolicy, from, to container.ProgramID) error {
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

	// we pull programs before locking
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
		if _, err := ag.startProcess(to); err != nil {
			return fmt.Errorf("cycle loop failed to start: %v", err)
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
		if err := ag.client.Programs().Remove(mproc.proc.Program().ID()); err != nil {
			ag.handleError(fmt.Errorf("cleaning up no longer used program %v, %v", prgmID, err))
		}
	}
}

func (ag *Agent) handleError(err error) {
	log.Err(err).Error("unexpected error")
}
