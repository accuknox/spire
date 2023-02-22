package telemetry

import (
	"github.com/accuknox/spire/pkg/common/version"
)

func EmitVersion(m Metrics) {
	m.SetGaugeWithLabels([]string{"started"}, 1, []Label{
		{Name: "version", Value: version.Version()},
	})
}
