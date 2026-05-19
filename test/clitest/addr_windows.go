//go:build windows

package clitest

import (
	"net"

	"github.com/accuknox/spire/pkg/common/namedpipe"
)

func GetAddr(addr net.Addr) string {
	return namedpipe.GetPipeName(addr.String())
}
