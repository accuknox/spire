package workloadattestor

import (
	"context"

	"github.com/spiffe/spire/pkg/common/plugin"
	"github.com/spiffe/spire/proto/spire/common"
	workloadattestorv1 "github.com/vishnusomank/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
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
