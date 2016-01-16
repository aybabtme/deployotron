package main

import (
	"time"

	"github.com/aybabtme/log"
)

const (
	appName = "supervisord"
)

func main() {
	ll := log.KV("app", appName)
	ll.Info("starting")
	for range time.Tick(5 * time.Second) {
		ll.Info("running...")
	}
}
