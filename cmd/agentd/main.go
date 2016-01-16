package main

import (
	"os"

	"github.com/aybabtme/log"
	"github.com/fsouza/go-dockerclient"
)

const (
	appName = "agentd"
)

func main() {
	ll := log.KV("app", appName)
	ll.Info("starting")

	dk, err := docker.NewClient(os.Getenv("DOCKERD_PORT"))
	if err != nil {
		log.Err(err).Fatal("can't create docker client")
	}

	if err := dk.Ping(); err != nil {
		log.Err(err).Fatal("docker is unreachable")
	}
}
