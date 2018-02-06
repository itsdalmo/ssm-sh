package command

import (
	"fmt"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
	"os"
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

	targets, err := targetFlagHelper(command.SsmOpts)
	if err != nil {
		return errors.Wrap(err, "Failed to set targets")
	}
	fmt.Printf("Initialized with targets: %s\n", targets)
	fmt.Printf("Use ctrl-c to abort the command early.\n\n")

	// Start the command
	commandId, err := m.Run(targets, strings.Join(args, " "))
	if err != nil {
		return errors.Wrap(err, "Failed to Run command")
	}

	// Catch sigterms to gracefully shut down
	abort := interruptHandler()
	m.Output(os.Stdout, targets, commandId, abort)

	return nil
}
