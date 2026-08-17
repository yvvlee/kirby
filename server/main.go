package main

import (
	"os"

	"github.com/yvvlee/kirby/server/cmd/kirby"
)

func main() {
	if err := kirby.Execute(); err != nil {
		os.Exit(1)
	}
}
