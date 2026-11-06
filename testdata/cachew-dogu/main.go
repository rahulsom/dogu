// Integration-test entry point: run the real CLI with all HTTP traffic routed
// through Cachew, including providers with hard-coded discovery endpoints.
package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/dogu-dev/dogu/internal/testsupport"
	"github.com/dogu-dev/dogu/pkg/cli"
)

func main() {
	transport, err := testsupport.CachewTransport(os.Getenv("CACHEW_URL"))
	if err == nil {
		http.DefaultTransport = transport
		err = cli.Run(os.Args[1:])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "dogu error:", err)
		os.Exit(1)
	}
}
