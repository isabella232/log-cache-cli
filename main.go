package main

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/log-cache-cli/internal/command"
)

type LogCacheCLI struct{}

var commands = map[string]command.Command{
	"tail":     command.Tail,
	"log-meta": command.Meta,
}

func (c *LogCacheCLI) Run(conn plugin.CliConnection, args []string) {
	if len(args) == 1 && args[0] == "CLI-MESSAGE-UNINSTALL" {
		// someone's uninstalling the plugin, but we don't need to clean up
		return
	}

	if len(args) < 1 {
		log.Fatalf("Expected at least 1 argument, but got %d.", len(args))
	}

	commands["log-query"] = func(ctx context.Context, cli plugin.CliConnection, args []string, c command.HTTPClient, log command.Logger, tableWriter io.Writer) {
		command.Query(
			ctx,
			cli,
			func(sourceID string, start, end time.Time) []string {
				var buf linesWriter
				command.Tail(
					ctx,
					cli,
					[]string{
						sourceID,
						"--start-time",
						strconv.FormatInt(start.UnixNano(), 10),
						"--end-time",
						strconv.FormatInt(end.UnixNano(), 10),
						"--json",
						"--lines", "1000",
					},
					c,
					log,
					&buf,
				)

				return buf.lines
			},
			args,
			c,
			log,
			tableWriter,
		)
	}

	skipSSL, err := conn.IsSSLDisabled()
	if err != nil {
		log.Fatalf("%s", err)
	}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: skipSSL,
	}

	op, ok := commands[args[0]]
	if !ok {
		log.Fatalf("Unknown Log Cache command: %s", args[0])
	}
	op(context.Background(), conn, args[1:], http.DefaultClient, log.New(os.Stderr, "", 0), os.Stdout)
}

func (c *LogCacheCLI) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "Log Cache CLI Plugin",
		Commands: []plugin.Command{
			{
				Name:     "tail",
				HelpText: "Output logs for a source-id/app",
				UsageDetails: plugin.Usage{
					Usage: `tail [options] <source-id/app>`,
					Options: map[string]string{
						"end-time":      "End of query range in UNIX nanoseconds.",
						"envelope-type": "Envelope type filter. Available filters: 'log', 'counter', 'gauge', 'timer', and 'event'.",
						"follow, -f":    "Output appended to stdout as logs are egressed.",
						"json":          "Output envelopes in JSON format.",
						"lines, -n":     "Number of envelopes to return. Default is 10.",
						"start-time":    "Start of query range in UNIX nanoseconds.",
						"counter-name":  "Counter name filter (implies --envelope-type=counter).",
						"gauge-name":    "Gauge name filter (implies --envelope-type=gauge).",
					},
				},
			},
			{
				Name:     "log-query",
				HelpText: "Output results for a PromQL",
				UsageDetails: plugin.Usage{
					Usage: `log-query [options] <promQL>`,
					Options: map[string]string{
						"end-time":   "End of query range in UNIX nanoseconds.",
						"start-time": "Start of query range in UNIX nanoseconds.",
					},
				},
			},
			{
				Name:     "log-meta",
				HelpText: "Show all available meta information",
				UsageDetails: plugin.Usage{
					Usage: "log-meta",
				},
			},
		},
	}
}

func main() {
	plugin.Start(&LogCacheCLI{})
}

type linesWriter struct {
	lines []string
}

func (w *linesWriter) Write(data []byte) (int, error) {
	w.lines = append(w.lines, string(data))
	return len(data), nil
}
