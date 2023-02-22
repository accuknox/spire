package workloadattestor

import (
	"context"

	workloadattestorv1 "github.com/accuknox/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	"github.com/accuknox/spire/pkg/common/plugin"
	"github.com/accuknox/spire/proto/spire/common"
)

type V1 struct {
	plugin.Facade
	workloadattestorv1.WorkloadAttestorPluginClient
}

func (v1 *V1) Attest(ctx context.Context, pid int, meta map[string]string) ([]*common.Selector, error) {
	resp, err := v1.WorkloadAttestorPluginClient.Attest(ctx, &workloadattestorv1.AttestRequest{
		Pid:  int32(pid),
		Meta: meta,
	})
	if err != nil {
		return nil, v1.WrapErr(err)
	}

	var selectors []*common.Selector
	if resp.SelectorValues != nil {
		selectors = make([]*common.Selector, 0, len(resp.SelectorValues))
		for _, selectorValue := range resp.SelectorValues {
			selectors = append(selectors, &common.Selector{
				Type:  v1.Name(),
				Value: selectorValue,
			})
		}
	}
	return selectors, nil
}
