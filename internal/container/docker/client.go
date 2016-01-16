package docker

import (
	"fmt"

	"github.com/aybabtme/deployotron/internal/container"
	"github.com/fsouza/go-dockerclient"
)

type client struct {
	dk *docker.Client
}

// New returns a container.Client implemented by Docker.
func New(endpoint string) (container.Client, error) {
	dk, err := docker.NewClient(endpoint)
	if err != nil {
		return nil, fmt.Errorf("can't create docker client: %v", err)
	}
	if err := dk.Ping(); err != nil {
		return nil, fmt.Errorf("can't ping docker: %v", err)
	}
	return &client{dk: dk}, nil
}

func (cl *client) ListImages() ([]container.Image, error) {
	panic("lol")
}

func (cl *client) ListContainers() ([]container.Container, error) {
	panic("lol")
}
