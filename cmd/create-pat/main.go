// Command create-pat creates an Azure DevOps personal access token for the user signed in to the
// Azure CLI and prints the token on stdout.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/skoszewski/ado-mcp/internal/ado"
	"github.com/skoszewski/ado-mcp/internal/clioutput"
)

const usageText = `Usage: create-pat --organization <org> [flags]

Create an Azure DevOps personal access token for the user signed in to the Azure CLI, and show
it with its scope, expiry and authorization ID. The token is read-only, with the Project and
Team, Build and Code read scopes ado-mcp needs, unless --full-scope is given.

Flags:
  -O, --organization <org>  Azure DevOps organization name or https://dev.azure.com/<org> URL;
                            required
      --name <name>         display name of the token (default: ado-mcp)
      --days <n>            number of days the token is valid for (default: 30)
      --full-scope          grant full access instead of the read-only scopes
      --bare                print only the token on stdout, for use in scripts
  -h, --help                show this help

Examples:
  create-pat -O myorg
  create-pat -O myorg --days 7 --name ado-mcp-ci
  create-pat -O myorg --full-scope
  echo "AZURE_DEVOPS_PAT=$(create-pat -O myorg --bare)" > .env
`

type options struct {
	organization string
	name         string
	days         int
	fullScope    bool
	bare         bool
}

func parseFlags() options {
	var opts options
	flags := flag.CommandLine
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usageText)
	}
	for _, name := range []string{"organization", "O"} {
		flags.StringVar(&opts.organization, name, "", "")
	}
	flags.StringVar(&opts.name, "name", "ado-mcp", "")
	flags.IntVar(&opts.days, "days", 30, "")
	flags.BoolVar(&opts.fullScope, "full-scope", false, "")
	flags.BoolVar(&opts.bare, "bare", false, "")
	flag.Parse()
	return opts
}

func main() {
	if err := run(parseFlags()); err != nil {
		clioutput.Fail(err)
	}
}

func run(opts options) error {
	if opts.organization == "" {
		return errors.New("--organization is required")
	}
	if opts.days < 1 {
		return errors.New("--days must be at least 1")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	authorizer, err := ado.NewAzureCLIAuthorizer()
	if err != nil {
		return err
	}
	client := &ado.Client{HTTP: &http.Client{Timeout: time.Minute}, Auth: authorizer}

	scope := ado.ReadOnlyScopes
	if opts.fullScope {
		scope = ado.FullScope
	}
	pat, err := client.CreatePAT(ctx, opts.organization, ado.PATRequest{
		DisplayName: opts.name,
		Scope:       scope,
		ValidTo:     time.Now().UTC().AddDate(0, 0, opts.days),
	})
	if err != nil {
		return err
	}

	if opts.bare {
		fmt.Println(pat.Token)
		return nil
	}
	clioutput.Success("Created personal access token '%s' in %s", pat.DisplayName, ado.NormalizeOrgURL(opts.organization))
	clioutput.Table([][]string{
		{"Scope:", pat.Scope},
		{"Valid to:", pat.ValidTo},
		{"Authorization ID:", pat.AuthorizationID},
		{"PAT:", pat.Token},
	})
	return nil
}
