package command

import (
	"fmt"
	"os"
)

// CommandVersion (set from main.go)
var CommandVersion string

func init() {
	Command.Version = func() {
		fmt.Println(CommandVersion)
		os.Exit(0)
	}
}
