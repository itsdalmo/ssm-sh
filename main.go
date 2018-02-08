package main

import (
	"github.com/itsdalmo/ssm-sh/command"
	"github.com/jessevdk/go-flags"
	"os"
)

func main() {
	_, err := flags.Parse(&command.Command)
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}
}
