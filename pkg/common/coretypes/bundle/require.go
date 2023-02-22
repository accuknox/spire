package bundle

import (
	"github.com/spiffe/spire/proto/spire/common"
	plugintypes "github.com/vishnusomank/spire-plugin-sdk/proto/spire/plugin/types"
)

func RequireToCommonFromPluginProto(pb *plugintypes.Bundle) *common.Bundle {
	out, err := ToCommonFromPluginProto(pb)
	panicOnError(err)
	return out
}

func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}
