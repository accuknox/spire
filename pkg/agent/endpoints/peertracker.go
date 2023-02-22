package endpoints

import (
	"context"

	attestor "github.com/accuknox/spire/pkg/agent/attestor/workload"
	"github.com/accuknox/spire/pkg/common/peertracker"
	"github.com/accuknox/spire/proto/spire/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PeerTrackerAttestor struct {
	Attestor attestor.Attestor
}

func (a PeerTrackerAttestor) Attest(ctx context.Context) ([]*common.Selector, error) {
	watcher, ok := peertracker.WatcherFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Internal, "peer tracker watcher missing from context")
	}

	var meta map[string]string

	v := ctx.Value("meta")
	if v != nil {
		meta = v.(map[string]string)
	}

	selectors := a.Attestor.Attest(ctx, int(watcher.PID()), meta)

	// Ensure that the original caller is still alive so that we know we didn't
	// attest some other process that happened to be assigned the original PID
	if err := watcher.IsAlive(); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "could not verify existence of the original caller: %v", err)
	}

	return selectors, nil
}
