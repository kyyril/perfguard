package main

import (
	"context"
	"os"

	"github.com/kyyril/perfguard/internal/cli"
)

func main() {
	os.Exit(cli.Execute(context.Background()))
}
