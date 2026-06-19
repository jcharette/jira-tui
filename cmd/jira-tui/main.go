package main

import (
	"fmt"
	"os"

	"github.com/jcharette/jira-tui/internal/app"
)

func main() {
	if err := app.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
