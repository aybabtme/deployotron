package main

import (
	"flag"
	"net"
	"time"

	"github.com/aybabtme/deployotron/internal/agent"
	"github.com/aybabtme/deployotron/internal/container/osprocess"
	"github.com/aybabtme/deployotron/internal/rpc"

	"github.com/aybabtme/log"
)

const (
	appName = "agentd"
)

func main() {
	supervisord := flag.String("supervisord", "127.0.0.1:1337", "address where the supervisor can be reached")
	flag.Parse()

	policy := agent.PolicyAllAtOnce()
	policy = agent.PolicyStartBeforeStop(policy)
	policy = agent.PolicyStopTimeout(policy, time.Second)

	ll := log.KV("app", appName)
	ll.Info("starting")
	defer ll.Info("all done")

	client := osprocess.New(osprocess.NopInstaller())
	// client = container.Log(client, log.KV("container", "osprocess"))

	ag := agent.New(client)

	for {
		cc, err := net.DialTimeout("tcp", *supervisord, 10*time.Second)
		if err != nil {
			ll.Err(err).Error("can't dial supervisord")
			continue
		}

		if err := rpc.OperateAgent(ag, client, cc); err != nil {
			ll.Err(err).Error("can't operate agent over RPC")
		}
	}
}
