//!build
package main

import (
	"time"

	"github.com/aybabtme/deployotron/internal/agent"
	"github.com/aybabtme/deployotron/internal/container/osprocess"
	// "github.com/aybabtme/deployotron/internal/container/docker"

	"github.com/aybabtme/log"
)

const (
	appName = "containerdriver"
)

func main() {
	policy := agent.PolicyAllAtOnce()
	policy = agent.PolicyStartBeforeStop(policy)
	policy = agent.PolicyStopTimeout(policy, time.Second)

	ll := log.KV("app", appName)
	ll.Info("starting")

	// client, err := docker.New(os.Getenv("DOCKERD_PORT"), "")
	// if err != nil {
	// 	log.Err(err).Fatal("can't create docker client")
	// }
	// client = container.Log(client, log.KV("container", "docker"))

	client := osprocess.New(osprocess.NopInstaller())
	// client = container.Log(client, log.KV("container", "osprocess"))

	img, err := client.ProgramID("echoer v1")
	if err != nil {
		ll.Err(err).Fatal("can't create program ID")
	}
	ll = ll.KV("program.id", img)

	ag := agent.New(client)
	ll.Info("starting program")
	if _, err := ag.StartProcess(img); err != nil {
		ll.Err(err).Fatal("couldn't start image")
	}
	time.Sleep(3 * time.Second)

	if _, err := ag.StartProcess(img); err != nil {
		ll.Err(err).Fatal("couldn't start image")
	}
	time.Sleep(3 * time.Second)

	ll.Info("restarting")
	if err := ag.RestartProgram(policy, img); err != nil {
		ll.Err(err).Fatal("couldn't restart image")
	}

	ll.Info("restarted, running")
	time.Sleep(3 * time.Second)

	newImg, err := client.ProgramID("echoer v2")
	if err != nil {
		ll.Err(err).Fatal("can't create program ID")
	}
	ll.Info("upgrading")
	if err := ag.UpgradeProgram(policy, img, newImg); err != nil {
		ll.Err(err).Fatal("couldn't restart image")
	}

	ll.Info("upgraded, running")
	time.Sleep(3 * time.Second)

	ll.Info("stopping all processes")
	if err := ag.StopProgram(newImg, 10*time.Second); err != nil {
		ll.Err(err).Fatal("couldn't stop image")
	}
	ll.Info("all done!")
}
