package rpc

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/aybabtme/deployotron/internal/agent"
	"github.com/aybabtme/deployotron/internal/container"
)

// A RemoteAgent that is exposed over RPC.
type RemoteAgent interface {
	StartProcess(*StartProcessReq) (*StartProcessRes, error)
	StopProcess(*StopProcessReq) (*StopProcessRes, error)
}

// RepresentAgent exposes a RemoteAgent from a bidirectional stream.
func RepresentAgent(r io.ReadWriteCloser, provider container.ProgramProvider) RemoteAgent {
	return &representant{
		provider: provider,
		sendMsg:  json.NewEncoder(r).Encode,
		readMsg:  json.NewDecoder(r).Decode,
	}
}

// OperateAgent operates an Agent over a bidirectional stream.
func OperateAgent(agent *agent.Agent, provider container.ProgramProvider, r io.ReadWriteCloser) error {
	op := &operator{
		agent:    agent,
		provider: provider,
		readMsg:  json.NewDecoder(r).Decode,
		sendMsg:  json.NewEncoder(r).Encode,
	}
	return op.service()
}

// Method definitions are in this order:
//    const methodName = "rpc/Agent.MethodName"
//
//    type (
//    	Req struct {Arg1 string}
//    	Res struct {Res1 string}
//    )
//
//    func (rep *representant) MethodName(*Req)      (*Res, error)
//    func (op *operator)      MethodName(*Req,*Res) (error)

func init() {
	rpcContract[methodStartProcess] = func(op *operator) (method methodCall, req interface{}) {
		return op.StartProcess, new(StartProcessReq)
	}
}

const methodStartProcess = "rpc/agent.StartProcess"

type (
	// StartProcessReq is an RPC request
	StartProcessReq struct {
		ProgramName string `json:"program_name"`
	}
	// StartProcessRes is an RPC response
	StartProcessRes struct {
		ProcessID container.ProcessID `json:"process_id"`
	}
)

func (rep *representant) StartProcess(req *StartProcessReq) (*StartProcessRes, error) {
	res := new(StartProcessRes)
	return res, rep.call(methodStartProcess, req, res)
}

func (op *operator) StartProcess(r interface{}) (interface{}, error) {
	req := r.(*StartProcessReq)
	prgmID := op.provider.ProgramID(req.ProgramName)
	proc, err := op.agent.StartProcess(prgmID)
	if err != nil {
		return nil, err
	}
	return &StartProcessRes{ProcessID: proc}, nil
}

func init() {
	rpcContract[methodStopProcess] = func(op *operator) (method methodCall, req interface{}) {
		r := new(StopProcessReq)
		return op.StopProcess, r
	}
}

const methodStopProcess = "rpc/agent.StopProcess"

type (
	// StopProcessReq is an RPC request
	StopProcessReq struct {
		ProcessID container.ProcessID `json:"process_id"`
		Timeout   time.Duration       `json:"timeout"`
	}
	// StopProcessRes is an RPC response
	StopProcessRes struct{}
)

func (rep *representant) StopProcess(req *StopProcessReq) (*StopProcessRes, error) {
	res := new(StopProcessRes)
	return res, rep.call(methodStopProcess, req, res)
}

func (op *operator) StopProcess(r interface{}) (interface{}, error) {
	req := r.(*StopProcessReq)
	err := op.agent.StopProcess(req.ProcessID, req.Timeout)
	if err != nil {
		return nil, err
	}
	return &StopProcessRes{}, nil
}

const methodListAll = "rpc/agent.ListAll"

type (
	// ListAllReq is an RPC request
	ListAllReq struct{}
	// ListAllRes is an RPC response
	ListAllRes struct {
		Running map[container.ProgramID][]container.ProcessID
	}
)

func (rep *representant) ListAll(req *ListAllReq) (*ListAllRes, error) {
	res := new(ListAllRes)
	return res, rep.call(methodListAll, req, res)
}

func (op *operator) ListAll(r interface{}) (interface{}, error) {
	return &ListAllRes{Running: op.agent.ListAll()}, nil
}

/*
	rpc internal details
*/

type rpcClientReq struct {
	MethodName string      `json:"method_name"`
	Request    interface{} `json:"request"`
}

type rpcServerReq struct {
	MethodName string          `json:"method_name"`
	Request    json.RawMessage `json:"request"`
}

type rpcClientRes struct {
	Response json.RawMessage `json:"response"`
	Err      string          `json:"error"`
}

type rpcServerRes struct {
	Response interface{} `json:"response"`
	Err      string      `json:"error"`
}

type methodCall func(interface{}) (interface{}, error)

var rpcContract = make(map[string]func(op *operator) (method methodCall, req interface{}))

type representant struct {
	provider container.ProgramProvider
	sendMsg  func(v interface{}) error
	readMsg  func(v interface{}) error
}

func (rep *representant) call(method string, req, res interface{}) error {
	if err := rep.sendMsg(rpcClientReq{
		MethodName: method,
		Request:    req,
	}); err != nil {
		return fmt.Errorf("sending rpc request message: %v", err)
	}
	rpcRes := new(rpcClientRes)
	if err := rep.readMsg(rpcRes); err != nil {
		return fmt.Errorf("reading rpc response message: %v", err)
	}
	if rpcRes.Err != "" {
		return fmt.Errorf("error response: %s", rpcRes.Err)
	}
	if err := json.Unmarshal(rpcRes.Response, res); err != nil {
		return fmt.Errorf("unmarshalling response: %v", err)
	}
	return nil
}

type operator struct {
	agent    *agent.Agent
	provider container.ProgramProvider

	readMsg func(v interface{}) error
	sendMsg func(v interface{}) error
}

func (op *operator) service() error {
	for {
		rpcReq := new(rpcServerReq)
		if err := op.readMsg(rpcReq); err != nil {
			return fmt.Errorf("can't read message: %v", err)
		}
		rpcRes := new(rpcServerRes)

		// find where to dispatch
		dispatcher, ok := rpcContract[rpcReq.MethodName]
		if !ok {
			rpcRes.Err = "unsupported method"
			return op.sendMsg(rpcRes)
		}
		method, req := dispatcher(op)

		// decode the method's arguments
		if err := json.Unmarshal(rpcReq.Request, req); err != nil {
			rpcRes.Err = "internal error"
			if err := op.sendMsg(rpcRes); err != nil {
				return fmt.Errorf("sending unmarshal error: %v", err)
			}
			return err
		}

		// invoke the method
		res, err := method(req)
		if err != nil {
			rpcRes.Err = fmt.Sprintf("method call error: %v", err)
			if err := op.sendMsg(rpcRes); err != nil {
				return fmt.Errorf("sending method call error: %v", err)
			}
		}

		// encode the response
		rpcRes.Response = res
		if err := op.sendMsg(rpcRes); err != nil {
			return fmt.Errorf("sending method call response: %v", err)
		}
	}
}
