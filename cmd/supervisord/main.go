package main

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/aybabtme/deployotron/internal/container/osprocess"
	"github.com/aybabtme/deployotron/internal/rpc"
	"github.com/aybabtme/log"
)

const (
	appName = "supervisord"
)

func main() {
	ll := log.KV("app", appName)
	ll.Info("starting")

	l, err := net.Listen("tcp", ":1337")
	if err != nil {
		ll.Err(err).Fatal("can't listen")
	}
	defer l.Close()

	acceptAgents(ll.KV("listen.addr", l.Addr().String()), l)
}

func acceptAgents(ll *log.Log, l net.Listener) {
	ll.Info("listening for agents")
	for {
		cc, err := l.Accept()
		if err != nil {
			ll.Err(err).Info("can't accept")
			return
		}

		go handleAgent(ll.KV("agent.addr", cc.RemoteAddr().String()), cc)
	}
}

func handleAgent(ll *log.Log, cc io.ReadWriteCloser) {
	defer cc.Close()
	client := osprocess.New(nil)
	agent := rpc.RepresentAgent(cc, client)

	for i := 0; ; i++ {
		ll.Info("starting program")

		res, err := agent.StartProcess(&rpc.StartProcessReq{
			ProgramName: fmt.Sprintf("echoer v%d i was told to do this by the supervisor", i),
		})
		if err != nil {
			ll.Err(err).Error("couldn't send command to remote agent")
			return
		}
		ll.KV("process.id", res.ProcessID).Info("agent is running program")

		time.Sleep(3 * time.Second)

		ll.Info("stopping program")
		_, err = agent.StopProcess(&rpc.StopProcessReq{ProcessID: res.ProcessID, Timeout: time.Second})
		if err != nil {
			ll.Err(err).Error("couldn't send command to remote agent")
			return
		}
	}

}
