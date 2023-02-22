package svid

import (
	"time"

	"github.com/accuknox/go-spiffe/v2/spiffeid"
	"github.com/accuknox/spire/pkg/common/telemetry"
	"github.com/accuknox/spire/pkg/server/ca"
	"github.com/accuknox/spire/pkg/server/plugin/keymanager"
	"github.com/andres-erbsen/clock"
	"github.com/imkira/go-observer"
	"github.com/sirupsen/logrus"
)

const (
	DefaultRotatorInterval = 5 * time.Second
)

type RotatorConfig struct {
	Log         logrus.FieldLogger
	Metrics     telemetry.Metrics
	TrustDomain spiffeid.TrustDomain
	ServerCA    ca.ServerCA
	Clock       clock.Clock
	KeyType     keymanager.KeyType

	// How long to wait between expiry checks
	Interval time.Duration
}

func NewRotator(c *RotatorConfig) *Rotator {
	if c.Interval == 0 {
		c.Interval = DefaultRotatorInterval
	}
	if c.Clock == nil {
		c.Clock = clock.New()
	}

	return &Rotator{
		c:     c,
		state: observer.NewProperty(State{}),
	}
}
