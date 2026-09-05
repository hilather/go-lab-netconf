// Command labnetconf is the LabNETCONF process entrypoint.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/hilather/go-lab-netconf/internal/buildinfo"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		printUsage(stderr)
		return 2
	}
	switch args[1] {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	case "version", "-v", "--version":
		_, _ = fmt.Fprintln(stdout, buildinfo.Current().String())
		return 0
	case "validate":
		return validateCmd(args[2:], stdout, stderr)
	case "canonicalize":
		return canonicalizeCmd(args[2:], stdout, stderr)
	case "healthcheck":
		return healthcheckCmd(args[2:], stdout, stderr)
	case "serve", "mcp-stdio":
		return notImplemented(args[1], stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n", args[1])
		printUsage(stderr)
		return 2
	}
}

func notImplemented(name string, stderr io.Writer) int {
	_, _ = fmt.Fprintf(stderr, "labnetconf %s: not implemented\n", name)
	return 1
}

func printUsage(w io.Writer) {
	_, _ = io.WriteString(w, usageText)
}

const usageText = `usage: labnetconf <command>

LabNETCONF is a laboratory NETCONF 1.0/1.1 + RESTCONF device with
per-user profiles. validate and canonicalize load a fail-closed
labnetconf.dev/v1alpha1 document. serve binds SSH NETCONF and HTTP
RESTCONF. Management REST /v1, MCP /mcp, and the operator SPA at /
bind only when --management-listen is an address (default off).

Commands:
  version         print build and protocol metadata
  help            print this help
  validate        fail-closed YAML check (--config)
  canonicalize    emit canonical spec (--config)
  serve           bind NETCONF/RESTCONF (--config, --netconf-listen,
                  --restconf-listen, --management-listen)
  healthcheck     probe GET /v1/health/ready (--url)
  mcp-stdio       Streamable MCP over stdio (--config, --token-file)
`
