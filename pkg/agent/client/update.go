package client

import "github.com/accuknox/spire/proto/spire/common"

type Update struct {
	Entries map[string]*common.RegistrationEntry
	Bundles map[string]*common.Bundle
}
