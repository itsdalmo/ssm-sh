package command

import (
	"fmt"
	"os"
)

// CommandVersion (overridden in main.go)
var CommandVersion = "unknown"

func init() {
	Command.Version = func() {
		fmt.Println(CommandVersion)
		os.Exit(0)
	}
}
