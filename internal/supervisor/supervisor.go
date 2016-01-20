package supervisor

import (
	"fmt"
	"net"
	"sync"

	"github.com/aybabtme/deployotron/internal/container"
	"github.com/aybabtme/deployotron/internal/rpc"
	"github.com/aybabtme/log"
)

type program string

type address string

func addressFromNet(addr net.Addr) address {
	return address(addr.String())
}

// Definition defines what a bunch of machines should run.
type Definition struct {
	Machines map[address]stack `json:"machines"`
}

type stack struct {
	Programs []program `json:"programs"`
}

var testDefinition = &Definition{
	Machines: map[address]stack{
		"127.0.0.1:13371": {
			Programs: []program{
				"echoer program.1",
				"echoer program.2",
			},
		},
		"127.0.0.1:13372": {
			Programs: []program{
				"echoer program.2",
				"echoer program.3",
			},
		},
	},
}

// Supervisor tells a bunch of machines what to run, all the time.
type Supervisor struct {
	provider container.ProgramProvider
	dfn      Definition

	mu     sync.Mutex
	agents map[address]*agent
}

// DefineStack takes a definition and creates a supervisor that will make sure
// the definition is applied on a bunch of machines.
func DefineStack(dfn Definition, provider container.ProgramProvider) (*Supervisor, error) {
	sup := &Supervisor{dfn: dfn, provider: provider}
	return sup, nil
}

// Listen makes the supervisor accept incoming agents from machines to supervise.
func (sup *Supervisor) Listen(l net.Listener) error {
	for {
		cc, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accepting connections: %v", err)
		}
		sup.acceptAgent(cc)
	}
}

func (sup *Supervisor) acceptAgent(cc net.Conn) {
	sup.mu.Lock()
	defer sup.mu.Unlock()

	raddr := addressFromNet(cc.RemoteAddr())
	ll := log.KV("raddr", raddr)

	if _, ok := sup.agents[raddr]; ok {
		ll.Info("agent already joined")
		_ = cc.Close()
		return
	}
	ll.Info("new agent joined")

	agent := &agent{
		ll:       ll,
		addr:     raddr,
		client:   rpc.RepresentAgent(cc, sup.provider),
		cc:       cc,
		setState: make(chan stack),
	}
	sup.agents[raddr] = agent

	if s, ok := sup.dfn.Machines[raddr]; !ok {
		ll.Info("no stack defined for this agent")
	} else {
		go agent.enforceState(s)
	}
}

type agent struct {
	ll     *log.Log
	addr   address
	cc     net.Conn
	client rpc.RemoteAgent

	setState chan stack
}

func (ag *agent) enforceState(desired stack) {
	ag.setState <- desired
}

func (ag *agent) maintainStateForever(imDead func(addr address)) {
	defer ag.cc.Close()
	defer imDead(ag.addr)
	for desiredState := range ag.setState {
		actualState, err := ag.gatherCurrentState()
		if err != nil {
			ag.ll.Err(err).Error("failed to gather current state")
			return
		}
		_ = actualState
	}
}

func (ag *agent) gatherCurrentState() (stack, error) {
	current := stack{}
	// ag.client.ListAll()
	return current, nil
}
