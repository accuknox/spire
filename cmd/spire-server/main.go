package main

import (
	"os"

	"github.com/accuknox/spire/cmd/spire-server/cli"
	"github.com/accuknox/spire/pkg/common/entrypoint"
)

func main() {
	os.Exit(entrypoint.NewEntryPoint(new(cli.CLI).Run).Main())
}
