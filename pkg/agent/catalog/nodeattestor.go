package catalog

import (
	"github.com/accuknox/spire/pkg/agent/plugin/nodeattestor"
	"github.com/accuknox/spire/pkg/agent/plugin/nodeattestor/awsiid"
	"github.com/accuknox/spire/pkg/agent/plugin/nodeattestor/azuremsi"
	"github.com/accuknox/spire/pkg/agent/plugin/nodeattestor/gcpiit"
	"github.com/accuknox/spire/pkg/agent/plugin/nodeattestor/jointoken"
	"github.com/accuknox/spire/pkg/agent/plugin/nodeattestor/k8spsat"
	"github.com/accuknox/spire/pkg/agent/plugin/nodeattestor/k8ssat"
	"github.com/accuknox/spire/pkg/agent/plugin/nodeattestor/sshpop"
	"github.com/accuknox/spire/pkg/agent/plugin/nodeattestor/tpmdevid"
	"github.com/accuknox/spire/pkg/agent/plugin/nodeattestor/x509pop"
	"github.com/accuknox/spire/pkg/common/catalog"
)

type nodeAttestorRepository struct {
	nodeattestor.Repository
}

func (repo *nodeAttestorRepository) Binder() interface{} {
	return repo.SetNodeAttestor
}

func (repo *nodeAttestorRepository) Constraints() catalog.Constraints {
	return catalog.ExactlyOne()
}

func (repo *nodeAttestorRepository) Versions() []catalog.Version {
	return []catalog.Version{
		nodeAttestorV1{},
	}
}

func (repo *nodeAttestorRepository) BuiltIns() []catalog.BuiltIn {
	return []catalog.BuiltIn{
		awsiid.BuiltIn(),
		azuremsi.BuiltIn(),
		gcpiit.BuiltIn(),
		jointoken.BuiltIn(),
		k8spsat.BuiltIn(),
		k8ssat.BuiltIn(),
		sshpop.BuiltIn(),
		tpmdevid.BuiltIn(),
		x509pop.BuiltIn(),
	}
}

type nodeAttestorV1 struct{}

func (nodeAttestorV1) New() catalog.Facade { return new(nodeattestor.V1) }
func (nodeAttestorV1) Deprecated() bool    { return false }
