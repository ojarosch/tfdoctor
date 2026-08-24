package main

import (
	"os"

	"github.com/ojarosch/tfdoctor/internal/cli"
)

func main() {
	os.Exit(cli.Run())
}
