package command

import (
	"fmt"
)

type RunCommand struct {
	SsmOpts SsmOptions
}

func (command *RunCommand) Execute(args []string) error {
	fmt.Println("RUN COMMAND")
	return nil
}
