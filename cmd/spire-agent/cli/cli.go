package cli

import (
	"context"
	stdlog "log"

	"github.com/accuknox/spire/cmd/spire-agent/cli/api"
	"github.com/accuknox/spire/cmd/spire-agent/cli/healthcheck"
	"github.com/accuknox/spire/cmd/spire-agent/cli/run"
	"github.com/accuknox/spire/cmd/spire-agent/cli/validate"
	"github.com/accuknox/spire/pkg/common/log"
	"github.com/accuknox/spire/pkg/common/version"
	"github.com/mitchellh/cli"
)

type CLI struct {
	LogOptions         []log.Option
	AllowUnknownConfig bool
}

func (cc *CLI) Run(ctx context.Context, args []string) int {
	c := cli.NewCLI("spire-agent", version.Version())
	c.Args = args
	c.Commands = map[string]cli.CommandFactory{
		"api fetch": func() (cli.Command, error) {
			return api.NewFetchX509Command(), nil
		},
		"api fetch x509": func() (cli.Command, error) {
			return api.NewFetchX509Command(), nil
		},
		"api fetch jwt": func() (cli.Command, error) {
			return api.NewFetchJWTCommand(), nil
		},
		"api validate jwt": func() (cli.Command, error) {
			return api.NewValidateJWTCommand(), nil
		},
		"api watch": func() (cli.Command, error) {
			return &api.WatchCLI{}, nil
		},
		"run": func() (cli.Command, error) {
			return run.NewRunCommand(ctx, cc.LogOptions, cc.AllowUnknownConfig), nil
		},
		"healthcheck": func() (cli.Command, error) {
			return healthcheck.NewHealthCheckCommand(), nil
		},
		"validate": func() (cli.Command, error) {
			return validate.NewValidateCommand(), nil
		},
	}

	exitStatus, err := c.Run()
	if err != nil {
		stdlog.Println(err)
	}
	return exitStatus
}
