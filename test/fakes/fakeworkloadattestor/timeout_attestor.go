package fakeworkloadattestor

import (
	"context"
	"testing"

	workloadattestorv1 "github.com/accuknox/spire-plugin-sdk/proto/spire/plugin/agent/workloadattestor/v1"
	"github.com/accuknox/spire/pkg/agent/plugin/workloadattestor"
	"github.com/accuknox/spire/pkg/common/catalog"
	"github.com/accuknox/spire/test/plugintest"
)

func NewTimeoutAttestor(t *testing.T, name string, c chan struct{}) workloadattestor.WorkloadAttestor {
	server := workloadattestorv1.WorkloadAttestorPluginServer(&timeoutWorkloadAttestor{
		c: c,
	})
	wa := new(workloadattestor.V1)
	plugintest.Load(t, catalog.MakeBuiltIn(name, server), wa)
	return wa
}

type timeoutWorkloadAttestor struct {
	workloadattestorv1.UnimplementedWorkloadAttestorServer

	c chan struct{}
}

func (twa *timeoutWorkloadAttestor) Attest(_ context.Context, _ *workloadattestorv1.AttestRequest) (*workloadattestorv1.AttestResponse, error) {
	// Block on channel until test sends signal
	<-twa.c
	return &workloadattestorv1.AttestResponse{}, nil
}
