package osprocess

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/aybabtme/deployotron/internal/container"
	"github.com/pborman/uuid"
)

// Installer knows how to install programs in the $PATH.
type Installer interface {
	Install(name string) error
	Uninstall(name string) error
}

// NopInstaller doesn't install or uninstall anything.
func NopInstaller() Installer {
	return nopInstaller{}
}

type nopInstaller struct{}

func (nopInstaller) Install(name string) error   { return fmt.Errorf("nop-install can't install") }
func (nopInstaller) Uninstall(name string) error { return fmt.Errorf("nop-install can't uninstall") }

type client struct {
	installer Installer
	programs  container.ProgramSvc
	processes container.ProcessSvc
}

// New creates a client that spawns regular OS processes.
func New(installer Installer) container.Client {
	cl := &client{installer: installer}
	cl.programs = &programSvc{client: cl}
	cl.processes = &processSvc{client: cl}
	return cl
}

func (osproc *client) ProgramID(name string) container.ProgramID {
	return container.ProgramID(newProgramID(name))
}

func (osproc *client) Programs() container.ProgramSvc  { return osproc.programs }
func (osproc *client) Processes() container.ProcessSvc { return osproc.processes }

type programSvc struct {
	client *client
}

func (svc *programSvc) Pull(id container.ProgramID) (container.Program, error) {
	prgm, ok, _ := svc.Get(id)
	if ok {
		return prgm, nil
	}
	prgmID := checkProgramID(id)
	if err := svc.client.installer.Install(prgmID.Name()); err != nil {
		return nil, fmt.Errorf("installing program %q: %v", prgmID.Name(), err)
	}
	prgm, _, err := svc.Get(id)
	return prgm, err
}

func (svc *programSvc) Get(id container.ProgramID) (container.Program, bool, error) {
	prgmID := checkProgramID(id)

	argv := strings.Split(prgmID.Name(), " ")

	path, err := exec.LookPath(argv[0])
	if perr, ok := err.(*exec.Error); ok && perr.Err == exec.ErrNotFound {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}

	return program{id: prgmID, path: path, argv: argv[1:]}, true, nil
}

func (svc *programSvc) Remove(id container.ProgramID) error {
	prgmID := checkProgramID(id)
	return svc.client.installer.Uninstall(prgmID.Name())
}

type programID container.ProgramID

func checkProgramID(id container.ProgramID) programID {
	if !strings.Contains(string(id), "osprocess.program.") {
		panic(fmt.Sprintf("bad container.ProgramID, want osprocess got %#v", id))
	}
	return programID(id)
}

func newProgramID(name string) programID {
	return programID("osprocess.program." + name)
}

func (pid programID) Name() string {
	return strings.TrimPrefix(string(pid), "osprocess.program.")
}

type program struct {
	id   programID
	path string
	argv []string
}

func (prgm program) ID() container.ProgramID {
	return container.ProgramID(prgm.id)
}

func checkProgram(prgm container.Program) program {
	osPrgm, ok := prgm.(program)
	if !ok {
		panic(fmt.Sprintf("bad container.Program, want %T got %T", program{}, prgm))
	}
	return osPrgm
}

// process stuff

type processID string

func (pid processID) UUID() string {
	return strings.TrimPrefix(string(pid), "osprocess.process.")
}

func newProcessID(uuid string) processID {
	return processID("osprocess.process." + uuid)
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
	uuid := uuid.New()
	return &process{
		svc:  svc,
		prgm: osPrgm,
		id:   newProcessID(uuid),
		cmd:  svc.create(osPrgm.path, osPrgm.argv),
	}, nil
}

func (svc *processSvc) Remove(proc container.Process) error { return nil } // nothing to do

func (svc *processSvc) create(path string, argv []string) *exec.Cmd {
	cmd := exec.Command(path, argv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func (proc *process) ID() container.ProcessID    { return container.ProcessID(proc.id) }
func (proc *process) Program() container.Program { return proc.prgm }

func (proc *process) Start() error {
	if proc.cmd.Process != nil {
		if err := proc.cmd.Process.Release(); err != nil {
			return fmt.Errorf("releasing OS process before starting: %v", err)
		}

		proc.cmd = proc.svc.create(proc.prgm.path, proc.prgm.argv)
	}

	if err := proc.cmd.Start(); err != nil {
		return fmt.Errorf("starting OS process: %v", err)
	}
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
