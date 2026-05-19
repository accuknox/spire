package bundlepublisher

import (
	"context"

	"github.com/accuknox/spire/pkg/common/catalog"
	"github.com/accuknox/spire/proto/spire/common"
)

type BundlePublisher interface {
	catalog.PluginInfo

	PublishBundle(ctx context.Context, bundle *common.Bundle) error
}
