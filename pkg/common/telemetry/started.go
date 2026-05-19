package telemetry

import (
	"github.com/accuknox/go-spiffe/v2/spiffeid"
	"github.com/accuknox/spire/pkg/common/version"
)

func EmitStarted(m Metrics, td spiffeid.TrustDomain) {
	m.SetGaugeWithLabels([]string{"started"}, 1, []Label{
		{Name: "version", Value: version.Version()},
		{Name: TrustDomainID, Value: td.Name()},
	})
}
