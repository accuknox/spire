package svid

import (
	"testing"

	"github.com/spiffe/spire/test/testca"
	"github.com/stretchr/testify/require"
	"github.com/vishnusomank/go-spiffe/v2/spiffeid"
)

func TestObserverFunc(t *testing.T) {
	ca := testca.New(t, spiffeid.RequireTrustDomainFromString("example.org"))
	svid := ca.CreateX509SVID(spiffeid.RequireFromString("spiffe://example.org/agent"))
	var f ObserverFunc = func() State {
		return State{
			SVID: svid.Certificates,
			Key:  svid.PrivateKey,
		}
	}
	require.Equal(t, f(), f.State())
}
