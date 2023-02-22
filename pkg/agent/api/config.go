package api

import (
	"net"
	"time"

	"github.com/accuknox/go-spiffe/v2/spiffeid"
	attestor "github.com/accuknox/spire/pkg/agent/attestor/workload"
	"github.com/accuknox/spire/pkg/agent/manager"
	"github.com/accuknox/spire/pkg/common/peertracker"
	"github.com/sirupsen/logrus"
)

type Config struct {
	BindAddr net.Addr

	Manager manager.Manager

	Log logrus.FieldLogger

	// Agent trust domain
	TrustDomain spiffeid.TrustDomain

	Uptime func() time.Duration

	Attestor attestor.Attestor

	AuthorizedDelegates []string
}

func New(c *Config) *Endpoints {
	return &Endpoints{
		c: c,
		listener: &peertracker.ListenerFactory{
			Log: c.Log,
		},
	}
}
