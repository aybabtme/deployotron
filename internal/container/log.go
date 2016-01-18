package container

import (
	"time"

	"github.com/aybabtme/log"
)

type logger struct {
	wrap Client
	l    *log.Log
}

// Log wraps all calls to a client with log messages.
func Log(client Client, l *log.Log) Client {
	return &logger{wrap: client, l: l}
}

func (log *logger) Programs() ProgramSvc {
	return &logProgramSvc{wrap: log.wrap.Programs(), l: log.l}
}
func (log *logger) Processes() ProcessSvc {
	return &logProcessSvc{wrap: log.wrap.Processes(), l: log.l}
}

type logProgramSvc struct {
	wrap ProgramSvc
	l    *log.Log
}

func (log *logProgramSvc) Pull(id ProgramID) (Program, error) {
	ll := log.l.KV("program.id", id)
	ll.Info("pulling program")

	prgm, err := log.wrap.Pull(id)
	if err != nil {
		ll.Err(err).Error("failed pulling program")
	} else {
		ll.Info("done pulling program")
	}
	return prgm, err
}

func (log *logProgramSvc) Get(id ProgramID) (Program, bool, error) {
	ll := log.l.KV("program.id", id)
	ll.Info("getting program")

	prgm, ok, err := log.wrap.Get(id)
	if err != nil {
		ll.Err(err).Error("failed getting program")
	} else {
		ll.KV("program.found", ok).Info("done getting program")
	}
	return prgm, ok, err
}

func (log *logProgramSvc) Remove(id ProgramID) error {
	ll := log.l.KV("program.id", id)
	ll.Info("removing program")

	err := log.wrap.Remove(id)
	if err != nil {
		ll.Err(err).Error("failed removing program")
	}
	return err
}

type logProcessSvc struct {
	wrap ProcessSvc
	l    *log.Log
}

func (log *logProcessSvc) Create(prgm Program) (Process, error) {
	ll := log.l.KV("program.id", prgm.ID())
	ll.Info("creating process")

	proc, err := log.wrap.Create(prgm)
	if err != nil {
		ll.Err(err).Error("failed creating process")
		return nil, err
	}
	ll.Info("done creating process")
	return &logProcess{wrap: proc, l: ll}, err
}

func (log *logProcessSvc) Remove(proc Process) error {
	ll := log.l.KV("proc.id", proc.ID())
	ll.Info("removing process")

	err := log.wrap.Remove(proc)
	if err != nil {
		ll.Err(err).Error("failed removing process")
		return err
	}
	ll.Info("done removing process")
	return err
}

type logProcess struct {
	wrap Process
	l    *log.Log
}

func (log *logProcess) ID() ProcessID    { return log.wrap.ID() }
func (log *logProcess) Program() Program { return log.wrap.Program() }

func (log *logProcess) Start() error {
	log.l.Info("starting process")
	if err := log.wrap.Start(); err != nil {
		log.l.Err(err).Error("failed starting process")
		return err
	}
	log.l = log.l.KV("proc.id", log.ID())
	log.l.Info("done starting process")
	return nil
}

func (log *logProcess) Stop(timeout time.Duration) error {
	log.l.Info("stopping process")
	if err := log.wrap.Stop(timeout); err != nil {
		log.l.Err(err).Error("failed stopping process")
		return err
	}
	log.l.Info("done stopping process")
	return nil
}

func (log *logProcess) Kill() error {
	log.l.Info("killing process")
	if err := log.wrap.Kill(); err != nil {
		log.l.Err(err).Error("failed killing process")
		return err
	}
	log.l.Info("done killing process")
	return nil
}

func (log *logProcess) Wait() error {
	log.l.Info("waiting for process")
	if err := log.wrap.Wait(); err != nil {
		log.l.Err(err).Error("failed waiting for process")
		return err
	}
	log.l.Info("done waiting for process")
	return nil
}
