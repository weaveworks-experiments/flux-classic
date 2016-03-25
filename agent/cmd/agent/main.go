package main

import (
	log "github.com/Sirupsen/logrus"

	"github.com/weaveworks/flux/agent"
	"github.com/weaveworks/flux/common/daemon"
	"github.com/weaveworks/flux/common/version"
)

func main() {
	log.Infof("flux agent version %s", version.Version())
	daemon.ConfigsMain(&agent.AgentConfig{})
}
