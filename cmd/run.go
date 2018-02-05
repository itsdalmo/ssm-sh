package command

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
	"strings"
)

type RunCommand struct {
	SsmOpts SsmOptions
}

func (command *RunCommand) Execute(args []string) error {
	sess, err := newSession()
	if err != nil {
		return errors.Wrap(err, "Failed to create new AWS session")
	}

	m := manager.NewManager(
		sess,
		Cmd.AwsOpts.Region,
		command.SsmOpts.Frequency,
		command.SsmOpts.Timeout,
	)

	// Start the command
	commandId, err := m.Run(command.SsmOpts.Targets, strings.Join(args, " "))
	if err != nil {
		return errors.Wrap(err, "Failed to Run command")
	}

	// Await output
	header := color.New(color.Bold)
	output := make(chan manager.Output)
	go m.Output(command.SsmOpts.Targets, commandId, output)

	for o := range output {
		header.Printf("%s - %s:\n", o.InstanceId, o.Status)
		if o.Error != nil {
			fmt.Println(o.Error)
			continue
		}
		fmt.Println(o.Output)
		fmt.Println("")
	}

	return nil
}
