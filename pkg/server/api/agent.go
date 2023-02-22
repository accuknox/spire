package api

import (
	"errors"

	"github.com/accuknox/go-spiffe/v2/spiffeid"
	"github.com/accuknox/spire/proto/spire/common"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

func ProtoFromAttestedNode(n *common.AttestedNode) (*types.Agent, error) {
	if n == nil {
		return nil, errors.New("missing attested node")
	}

	spiffeID, err := spiffeid.FromString(n.SpiffeId)
	if err != nil {
		return nil, err
	}

	return &types.Agent{
		AttestationType:      n.AttestationDataType,
		Id:                   ProtoFromID(spiffeID),
		X509SvidExpiresAt:    n.CertNotAfter,
		X509SvidSerialNumber: n.CertSerialNumber,
		Banned:               n.CertSerialNumber == "",
		Selectors:            ProtoFromSelectors(n.Selectors),
	}, nil
}
