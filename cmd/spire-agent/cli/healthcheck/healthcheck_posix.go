//go:build !windows
// +build !windows

package healthcheck

import (
	"flag"
	"net"

	"github.com/accuknox/spire/cmd/spire-agent/cli/common"
	"github.com/accuknox/spire/pkg/common/util"
)

// healthCheckCommandOS has posix specific implementation
// that complements healthCheckCommand
type healthCheckCommandOS struct {
	socketPath string
}

func (c *healthCheckCommandOS) addOSFlags(flags *flag.FlagSet) {
	flags.StringVar(&c.socketPath, "socketPath", common.DefaultSocketPath, "Path to the SPIRE Agent API socket")
}

func (c *healthCheckCommandOS) getAddr() (net.Addr, error) {
	return util.GetUnixAddrWithAbsPath(c.socketPath)
}
