package fakeserverkeymanager

import (
	"testing"

	keymanagerv1 "github.com/accuknox/spire-plugin-sdk/proto/spire/plugin/server/keymanager/v1"
	"github.com/accuknox/spire/pkg/common/catalog"
	"github.com/accuknox/spire/pkg/server/plugin/keymanager"
	keymanagerbase "github.com/accuknox/spire/pkg/server/plugin/keymanager/base"
	"github.com/accuknox/spire/test/plugintest"
	"github.com/accuknox/spire/test/testkey"
)

func New(t *testing.T) keymanager.KeyManager {
	plugin := keyManager{
		Base: keymanagerbase.New(keymanagerbase.Config{
			Generator: &testkey.Generator{},
		}),
	}

	v1 := new(keymanager.V1)
	plugintest.Load(t, catalog.MakeBuiltIn("fake", keymanagerv1.KeyManagerPluginServer(plugin)), v1)
	return v1
}

type keyManager struct {
	*keymanagerbase.Base
}
