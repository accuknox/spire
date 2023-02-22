package workloadattestor

import (
	"context"

	"github.com/accuknox/spire/pkg/common/catalog"
	"github.com/accuknox/spire/proto/spire/common"
)

type WorkloadAttestor interface {
	catalog.PluginInfo

	Attest(ctx context.Context, pid int, meta map[string]string) ([]*common.Selector, error)
}
