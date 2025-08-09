package main

import (
	"fmt"
	"os"

	"github.com/wolfsTail/s3cli/internal/cli"
)

func main() {
	code, err := cli.Run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
	os.Exit(code)
}
