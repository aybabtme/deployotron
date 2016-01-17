package osprocess

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"github.com/aybabtme/deployotron/internal/container"
)

// Installer knows how to install programs in the $PATH.
type Installer interface {
	Install(name string) error
}

type client struct {
	installer Installer
	programs  container.ProgramSvc
	processes container.ProcessSvc
}

// New creates a client that spawns regular OS processes.
func New(installer Installer) container.Client {
	cl := &client{}
	cl.programs = &programSvc{client: cl}
	cl.processes = &processSvc{client: cl}
	return cl
}

func (dk *client) Programs() container.ProgramSvc  { return dk.programs }
func (dk *client) Processes() container.ProcessSvc { return dk.processes }

type programSvc struct {
	client *client
}

func (svc *programSvc) Pull(id container.ProgramID) (container.Program, error) {
	prgm, ok, _ := svc.Get(id)
	if ok {
		return prgm, nil
	}
	prgmID := checkProgramID(id)
	if err := svc.client.installer.Install(prgmID.name); err != nil {
		return nil, fmt.Errorf("installing program %q: %v", prgmID.name, err)
	}
	prgm, _, err := svc.Get(id)
	return prgm, err
}

func (svc *programSvc) Get(id container.ProgramID) (container.Program, bool, error) {
	prgmID := checkProgramID(id)

	path, err := exec.LookPath(prgmID.name)
	if perr, ok := err.(*exec.Error); ok && perr.Err == exec.ErrNotFound {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}

	return &program{id: prgmID, path: path}, true, nil
}

type programID struct {
	name string
}

// ProgramID returns a container.ProgramID made from a path.
func ProgramID(name string) container.ProgramID {
	return programID{name: name}
}

func checkProgramID(id container.ProgramID) programID {
	pid, ok := id.(programID)
	if !ok {
		panic(fmt.Sprintf("bad container.ProgramID, want %T got %T", programID{}, id))
	}
	return pid
}

type program struct {
	id   programID
	path string
}

func (prgm program) ID() container.ProgramID {
	return prgm.id
}

func checkProgram(prgm container.Program) program {
	osPrgm, ok := prgm.(program)
	if !ok {
		panic(fmt.Sprintf("bad container.Program, want %T got %T", program{}, prgm))
	}
	return osPrgm
}

// process stuff

type processID struct{ pid int }

func procIDFromCmd(cmd *exec.Cmd) processID {
	return processID{pid: cmd.Process.Pid}
}

type process struct {
	svc  *processSvc
	id   processID
	prgm program
	cmd  *exec.Cmd
}

type processSvc struct {
	client *client
}

func (svc *processSvc) Create(prgm container.Program) (container.Process, error) {
	osPrgm := checkProgram(prgm)
	cmd := exec.Command(osPrgm.path)
	var id processID
	if cmd.Process != nil {
		id = procIDFromCmd(cmd)
	}
	return &process{
		svc:  svc,
		prgm: osPrgm,
		id:   id,
		cmd:  cmd,
	}, nil
}

func (proc *process) ID() container.ProcessID    { return proc.id }
func (proc *process) Program() container.Program { return proc.prgm }

func (proc *process) Start() error {
	if err := proc.cmd.Start(); err != nil {
		return fmt.Errorf("starting OS process: %v", err)
	}
	proc.id = procIDFromCmd(proc.cmd)
	return nil
}

func (proc *process) Stop(timeout time.Duration) error {
	err := proc.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("stopping OS process with SIGTERM: %v", err)
	}
	return nil
}

func (proc *process) Kill() error {
	if err := proc.cmd.Process.Kill(); err != nil {
		return fmt.Errorf("killing OS process: %v", err)
	}
	return nil
}

func (proc *process) Wait() error {
	if err := proc.cmd.Wait(); err != nil {
		return fmt.Errorf("waiting for OS process: %v", err)
	}
	return nil
}