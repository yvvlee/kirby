package main

import (
	"fmt"
	"os"

	"github.com/yvvlee/kirby/server/cmd/kirby"
)

func main() {
	if err := kirby.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
