package main

import (
	"github.com/itsdalmo/ssm-sh/cmd"
	"github.com/jessevdk/go-flags"
	"os"
)

func main() {
	_, err := flags.Parse(&command.Cmd)
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}
}
