package docker

import (
	"fmt"
	"time"

	"github.com/aybabtme/deployotron/internal/container"
	"github.com/fsouza/go-dockerclient"
)

// New returns a container.Client implemented by Docker.
func New(endpoint, registry string) (container.Client, error) {
	dk, err := docker.NewClient(endpoint)
	if err != nil {
		return nil, fmt.Errorf("can't create docker client: %v", err)
	}
	if err := dk.Ping(); err != nil {
		return nil, fmt.Errorf("can't ping docker: %v", err)
	}
	cl := &client{dk: dk, registry: registry}
	cl.programs = &programSvc{client: cl}
	cl.processes = &processSvc{client: cl}
	return cl, nil
}

type client struct {
	dk       *docker.Client
	registry string
	auth     docker.AuthConfiguration

	programs  container.ProgramSvc
	processes container.ProcessSvc
}

func (dk *client) Programs() container.ProgramSvc  { return dk.programs }
func (dk *client) Processes() container.ProcessSvc { return dk.processes }

// program stuff

type programSvc struct {
	client *client
}

func (svc *programSvc) Pull(id container.ProgramID) (container.Program, error) {
	pid := checkProgramID(id)
	dk := svc.client.dk
	opts := docker.PullImageOptions{
		Repository: pid.ImageName(),
		Registry:   svc.client.registry,
	}
	auth := svc.client.auth
	if err := dk.PullImage(opts, auth); err != nil {
		return nil, fmt.Errorf("pulling docker image: %v", err)
	}
	img, err := dk.InspectImage(pid.ImageName())
	if err != nil {
		return nil, fmt.Errorf("inspecting docker image: %v", err)
	}
	return program{img: *img}, nil
}

func (svc *programSvc) Get(id container.ProgramID) (container.Program, bool, error) {
	pid := checkProgramID(id)
	dk := svc.client.dk
	img, err := dk.InspectImage(pid.ImageName())
	switch err {
	case docker.ErrNoSuchImage:
		return nil, false, nil
	case nil:
		return program{id: pid, img: *img}, true, nil
	default:
		return nil, false, fmt.Errorf("inspecting docker image: %v", err)
	}
}

type program struct {
	id  programID
	img docker.Image
}

func checkProgram(prgm container.Program) program {
	dkPrgm, ok := prgm.(program)
	if !ok {
		panic(fmt.Sprintf("bad container.Program, want %T got %T", program{}, prgm))
	}
	return dkPrgm
}

func (prgm program) ID() container.ProgramID {
	return prgm.id
}

type programID struct {
	imgName string
}

func checkProgramID(id container.ProgramID) programID {
	pid, ok := id.(programID)
	if !ok {
		panic(fmt.Sprintf("bad container.ProgramID, want %T got %T", programID{}, id))
	}
	return pid
}

func (pid programID) ImageName() string { return pid.imgName }

// process stuff

type processSvc struct {
	client *client
}

func (svc *processSvc) Create(prgm container.Program) (container.Process, error) {
	dk := svc.client.dk
	dkPrgm := checkProgram(prgm)
	opts := docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: dkPrgm.id.ImageName(),
		},
	}
	container, err := dk.CreateContainer(opts)
	if err != nil {
		return nil, fmt.Errorf("creating docker container: %v", err)
	}
	return &process{
		svc:       svc,
		id:        procIDFromContainer(container),
		prgm:      dkPrgm,
		container: container,
	}, nil
}

type processID struct{ id string }

func procIDFromContainer(dkCtnr *docker.Container) processID {
	return processID{id: dkCtnr.ID}
}

type process struct {
	svc       *processSvc
	id        processID
	prgm      program
	container *docker.Container
}

func (proc *process) ID() container.ProcessID    { return proc.id }
func (proc *process) Program() container.Program { return proc.prgm }

func (proc *process) Start() error {
	dk := proc.svc.client.dk
	if err := dk.StartContainer(proc.id.id, nil); err != nil {
		return fmt.Errorf("starting docker container: %v", err)
	}
	return nil
}

func (proc *process) Stop(timeout time.Duration) error {
	dk := proc.svc.client.dk
	timeoutSec := uint(timeout.Seconds())
	if err := dk.StopContainer(proc.id.id, timeoutSec); err != nil {
		return fmt.Errorf("stopping docker container: %v", err)
	}
	return nil
}

func (proc *process) Kill() error {
	dk := proc.svc.client.dk
	if err := dk.KillContainer(docker.KillContainerOptions{ID: proc.id.id}); err != nil {
		return fmt.Errorf("killing docker container: %v", err)
	}
	return nil
}

func (proc *process) Wait() error {
	dk := proc.svc.client.dk
	// TODO(antoine): maybe someone cares about the exit code one day
	if _, err := dk.WaitContainer(proc.id.id); err != nil {
		return fmt.Errorf("waiting on docker container: %v", err)
	}
	return nil
}
