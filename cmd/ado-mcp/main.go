// Command ado-mcp serves Azure DevOps pipeline inventory, run history, run logs and Git
// repositories as MCP tools, over Streamable HTTP or stdio.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/skoszewski/ado-mcp/internal/ado"
	"github.com/skoszewski/ado-mcp/internal/tools"
)

// version is set at build time with -ldflags "-X main.version=<version>".
var version = "dev"

const (
	serverName      = "ado-mcp"
	shutdownTimeout = 15 * time.Second
	httpTimeout     = 2 * time.Minute
	debugToolCalls  = 1
	debugRequests   = 2
	debugSDK        = 3
)

const usageText = `Usage: ado-mcp [flags]

Run an MCP server exposing Azure DevOps pipeline inventory and run history -- projects, folders,
pipelines, runs, run timelines and run logs -- plus the Git repositories those runs build.

Every tool call names the organization and project it applies to, and reaches whatever the
authenticated identity is authorized for in Azure DevOps.

Authentication, first match wins:
  AZURE_DEVOPS_PAT                                          personal access token
  AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET     service principal client secret
  otherwise                                                 the signed-in Azure CLI

Flags:
`

const usageExamples = `
Examples:
  ado-mcp
  ado-mcp --port 9000 --path /ado
  ado-mcp --transport stdio
`

// debugLevel is the --debug flag: a bare --debug sets level 1, --debug=N sets level N.
type debugLevel int

func (d *debugLevel) String() string {
	return strconv.Itoa(int(*d))
}

func (d *debugLevel) Set(value string) error {
	if value == "true" {
		*d = debugToolCalls
		return nil
	}
	level, err := strconv.Atoi(value)
	if err != nil || level < 0 || level > debugSDK {
		return errors.New("must be 0, 1, 2 or 3")
	}
	*d = debugLevel(level)
	return nil
}

func (d *debugLevel) IsBoolFlag() bool {
	return true
}

type options struct {
	transport     string
	host          string
	port          int
	path          string
	maxLogLines   int
	optimizations tools.Optimizations
	debug         debugLevel
}

func parseFlags() options {
	opts := options{optimizations: tools.DefaultOptimizations()}
	flags := flag.CommandLine
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), usageText)
		flags.PrintDefaults()
		fmt.Fprint(flags.Output(), usageExamples)
	}

	flags.StringVar(&opts.transport, "transport", "http", "MCP transport: http (Streamable HTTP) or stdio")
	flags.StringVar(&opts.host, "host", "127.0.0.1", "address to bind the HTTP server to")
	flags.IntVar(&opts.port, "port", 8888, "port the HTTP server listens on")
	flags.StringVar(&opts.path, "path", "/mcp", "URL path the MCP endpoint is served at")
	flags.IntVar(&opts.maxLogLines, "max-log-lines", 2000, "maximum log lines ado_get_run_log may return in one call")
	flags.Func("optimize", `comma-separated settings the tools adapt their behaviour to, as switches named on their own
and keys given as name=value, e.g. 'small-model,log-type=task'. small-model suits a client model
that does not reliably page through a long result: ado_get_run_log then returns the whole log,
or its last --max-log-lines lines when it is longer than that, instead of the page the call
asked for. log-type (job, task or all; job by default) sets which logs ado_list_run_logs reports
when the call does not name one itself -- task lists each step's own log, which is far smaller
than a job's combined output`, func(value string) error {
		optimizations, err := tools.ParseOptimizations(value)
		opts.optimizations = optimizations
		return err
	})
	flags.Var(&opts.debug, "debug", `debug output level, given as --debug or --debug=N: 1 (the level of a bare --debug)
logs every tool call with the arguments it was given, 2 adds one line per incoming HTTP
request, and 3 adds the MCP library's own logging, which dumps raw protocol traffic`)

	flag.Parse()
	return opts
}

func main() {
	opts := parseFlags()
	level := slog.LevelInfo
	if opts.debug > 0 {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	if err := run(opts); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if opts.transport != "http" && opts.transport != "stdio" {
		return fmt.Errorf("unknown --transport %q; use http or stdio", opts.transport)
	}
	if !strings.HasPrefix(opts.path, "/") {
		return fmt.Errorf("--path %q must start with /", opts.path)
	}
	if opts.maxLogLines < 1 {
		return errors.New("--max-log-lines must be at least 1")
	}

	var sdkLogger *slog.Logger
	if opts.debug >= debugSDK {
		sdkLogger = slog.Default()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	authorizer, err := ado.NewAuthorizer(os.Getenv)
	if err != nil {
		return err
	}
	client := &ado.Client{HTTP: &http.Client{Timeout: httpTimeout}, Auth: authorizer}

	server := mcp.NewServer(
		&mcp.Implementation{Name: serverName, Version: version},
		&mcp.ServerOptions{Instructions: tools.Instructions, Logger: sdkLogger},
	)
	toolNames := tools.Register(server, client, opts.maxLogLines, opts.optimizations)
	slog.Info("starting server", "version", version, "transport", opts.transport,
		"authentication", client.Auth.Method(), "optimize", opts.optimizations.String(),
		"debug", int(opts.debug), "tools", strings.Join(toolNames, ","))

	if opts.transport == "stdio" {
		return server.Run(ctx, &mcp.StdioTransport{})
	}
	return serveHTTP(ctx, server, opts, sdkLogger)
}

// serveHTTP serves server as a stateless Streamable HTTP endpoint until ctx is cancelled.
func serveHTTP(ctx context.Context, server *mcp.Server, opts options, sdkLogger *slog.Logger) error {
	var handler http.Handler = mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, Logger: sdkLogger},
	)
	if opts.debug >= debugRequests {
		next := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slog.Info("http request", "method", r.Method, "path", r.URL.Path, "remote", r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
	mux := http.NewServeMux()
	mux.Handle(opts.path, handler)

	httpServer := &http.Server{Addr: net.JoinHostPort(opts.host, strconv.Itoa(opts.port)), Handler: mux}
	slog.Info("listening", "endpoint", "http://"+httpServer.Addr+opts.path)
	failed := make(chan error, 1)
	go func() {
		failed <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}
	slog.Info("stopping server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
