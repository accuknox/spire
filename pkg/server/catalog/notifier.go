package catalog

import (
	"github.com/accuknox/spire/pkg/common/catalog"
	"github.com/accuknox/spire/pkg/server/plugin/notifier"
	"github.com/accuknox/spire/pkg/server/plugin/notifier/gcsbundle"
	"github.com/accuknox/spire/pkg/server/plugin/notifier/k8sbundle"
)

type notifierRepository struct {
	notifier.Repository
}

func (repo *notifierRepository) Binder() interface{} {
	return repo.AddNotifier
}

func (repo *notifierRepository) Constraints() catalog.Constraints {
	return catalog.ZeroOrMore()
}

func (repo *notifierRepository) Versions() []catalog.Version {
	return []catalog.Version{
		notifierV1{},
	}
}

func (repo *notifierRepository) BuiltIns() []catalog.BuiltIn {
	return []catalog.BuiltIn{
		gcsbundle.BuiltIn(),
		k8sbundle.BuiltIn(),
	}
}

type notifierV1 struct{}

func (notifierV1) New() catalog.Facade { return new(notifier.V1) }
func (notifierV1) Deprecated() bool    { return false }
