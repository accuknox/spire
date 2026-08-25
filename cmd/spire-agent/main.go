package main

import (
	"fmt"
	"os"

	"github.com/accuknox/spire/cmd/spire-agent/cli"
	"github.com/accuknox/spire/pkg/common/entrypoint"
	"github.com/accuknox/spire/pkg/common/monitoring"
	"github.com/accuknox/spire/pkg/common/version"
)

func main() {
	os.Exit(run())
}

func run() int {
	monitor, err := monitoring.New("spire-agent", version.Version())
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
