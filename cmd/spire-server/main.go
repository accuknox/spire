package main

import (
	"fmt"
	"os"

	"github.com/spiffe/spire/cmd/spire-server/cli"
	"github.com/spiffe/spire/pkg/common/entrypoint"
	"github.com/spiffe/spire/pkg/common/monitoring"
	"github.com/spiffe/spire/pkg/common/version"
)

func main() {
	os.Exit(run())
}

func run() int {
	monitor, err := monitoring.New("spire-server", version.Version())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Sentry monitoring disabled: %v\n", err)
		monitor = monitoring.Disabled()
	}
	defer monitor.Flush()
	defer monitor.Recover()

	command := new(cli.CLI)
	command.LogOptions = append(command.LogOptions, monitor.LogOption())
	return entrypoint.NewEntryPoint(command.Run).Main()
}
