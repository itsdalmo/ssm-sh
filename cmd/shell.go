package command

import (
	"fmt"
)

type ShellCommand struct {
	SsmOpts SsmOptions
}

func (command *ShellCommand) Execute([]string) error {
	fmt.Println("SHELL COMMAND")
	return nil
}
