package command

import (
	"fmt"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
	"os"
)

type ShellCommand struct {
	SsmOpts SsmOptions
}

func (command *ShellCommand) Execute([]string) error {
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

	targets, err := targetFlagHelper(command.SsmOpts)
	if err != nil {
		return errors.Wrap(err, "Failed to set targets")
	}
	fmt.Printf("Initialized with targets: %s\n", targets)
	fmt.Printf("Type 'exit' to exit. Use ctrl-c to abort running commands.\n\n")

	// Channels to control user prompt
	input := make(chan string)
	prompt := make(chan bool)
	defer close(prompt)

	go userPrompt(input, prompt)
	abort := interruptHandler()

	for {
		// Request user input
		prompt <- true
		cmd := <-input

		// Exit if specified
		if cmd == "exit" {
			return nil
		}

		// Start command
		commandId, err := m.Run(targets, cmd)
		if err != nil {
			return errors.Wrap(err, "Failed to Run command")
		}
		m.Output(os.Stdout, targets, commandId, abort)
	}
}
