package main

import (
	"os"
	"time"

	"github.com/aybabtme/deployotron/internal/agent"
	"github.com/aybabtme/deployotron/internal/container"
	"github.com/aybabtme/deployotron/internal/container/docker"
	"github.com/aybabtme/log"
)

const (
	appName = "agentd"
)

func main() {
	ll := log.KV("app", appName)
	ll.Info("starting")

	dk, err := docker.New(os.Getenv("DOCKERD_PORT"), "")
	if err != nil {
		log.Err(err).Fatal("can't create docker client")
	}
	dk = container.Log(dk, log.KV("container", "docker"))

	img := docker.ProgramID("wtv")
	ll = ll.KV("program", img)

	ag := agent.New(dk)
	ll.Info("starting program")
	if err := ag.Start(img); err != nil {
		log.Err(err).Fatal("couldn't start image")
	}
	time.Sleep(60 * time.Second)
	if err := ag.StopAll(img, 10*time.Second); err != nil {
		log.Err(err).Fatal("couldn't stop image")
	}
}
