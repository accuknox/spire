package fakeagentkeymanager

import (
	"testing"

	"github.com/accuknox/spire/pkg/agent/plugin/keymanager"
	"github.com/accuknox/spire/pkg/agent/plugin/keymanager/disk"
	"github.com/accuknox/spire/pkg/agent/plugin/keymanager/memory"
	"github.com/accuknox/spire/test/plugintest"
	"github.com/accuknox/spire/test/testkey"
)

// New returns a fake key manager
func New(t *testing.T, dir string) keymanager.KeyManager {
	km := new(keymanager.V1)
	if dir != "" {
		plugintest.Load(t, disk.TestBuiltIn(&testkey.Generator{}), km, plugintest.Configuref("directory = %q", dir))
	} else {
		plugintest.Load(t, memory.TestBuiltIn(&testkey.Generator{}), km)
	}
	return km
}
