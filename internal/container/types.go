// Package container abstracts what we want to do with containers.
package container

import "time"

// A ProgramProvider can instantiate a ProgramID from a name.
type ProgramProvider interface {
	ProgramID(name string) ProgramID
}

// A Client knows how to do container stuff.
type Client interface {
	ProgramProvider
	Programs() ProgramSvc
	Processes() ProcessSvc
}

// An ProgramSvc exposes facilities to interact with programs.
type ProgramSvc interface {
	Pull(id ProgramID) (Program, error)
	Get(id ProgramID) (Program, bool, error)
	Remove(id ProgramID) error
}

// A ProcessSvc is a service to interact with processes.
type ProcessSvc interface {
	Create(Program) (Process, error)
	Remove(Process) error
}

// A ProgramID uniquely identifies a Program.
type ProgramID string

// A Program is the template of a Process.
type Program interface {
	ID() ProgramID
}

// A ProcessID uniquely idenfities a Process.
type ProcessID string

// A Process is the running execution of a program.
type Process interface {
	ID() ProcessID // The ID must be stable across restarts
	Program() Program
	Start() error
	Stop(time.Duration) error
	Kill() error
	Wait() error
}
