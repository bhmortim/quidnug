// Command quidnug runs a Quidnug protocol node.
//
// Configuration is loaded from environment variables, an optional
// config file (CONFIG_FILE env var, or ./config.yaml / /etc/quidnug/config.yaml),
// and built-in defaults — in that order of precedence. See
// config.example.yaml in the repository root for the full schema.
package main

import "github.com/quidnug/quidnug/internal/core"

func main() {
	core.Run()
}
