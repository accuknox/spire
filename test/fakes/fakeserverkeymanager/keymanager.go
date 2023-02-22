package fakeserverkeymanager

import (
	"testing"

	"github.com/spiffe/spire/pkg/common/catalog"
	"github.com/spiffe/spire/pkg/server/plugin/keymanager"
	keymanagerbase "github.com/spiffe/spire/pkg/server/plugin/keymanager/base"
	"github.com/spiffe/spire/test/plugintest"
	"github.com/spiffe/spire/test/testkey"
	keymanagerv1 "github.com/vishnusomank/spire-plugin-sdk/proto/spire/plugin/server/keymanager/v1"
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
