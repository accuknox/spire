package memory

import (
	"github.com/spiffe/spire/pkg/common/catalog"
	keymanagerbase "github.com/spiffe/spire/pkg/server/plugin/keymanager/base"
	keymanagerv1 "github.com/vishnusomank/spire-plugin-sdk/proto/spire/plugin/server/keymanager/v1"
)

type Generator = keymanagerbase.Generator

func BuiltIn() catalog.BuiltIn {
	return asBuiltIn(newKeyManager(nil))
}

func TestBuiltIn(generator Generator) catalog.BuiltIn {
	return asBuiltIn(newKeyManager(generator))
}

func asBuiltIn(p *KeyManager) catalog.BuiltIn {
	return catalog.MakeBuiltIn("memory", keymanagerv1.KeyManagerPluginServer(p))
}

type KeyManager struct {
	*keymanagerbase.Base
}

func newKeyManager(generator Generator) *KeyManager {
	return &KeyManager{
		Base: keymanagerbase.New(keymanagerbase.Config{
			Generator: generator,
		}),
	}
}
