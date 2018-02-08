package cmd

import (
	"bufio"
	"context"
	"fmt"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
	"os"
)

type ShellCommand struct {
	TargetOpts TargetOptions
}

func (command *ShellCommand) Execute([]string) error {
	sess, err := newSession()
	if err != nil {
		return errors.Wrap(err, "failed to create new aws session")
	}

	m := manager.NewManager(sess, Command.AwsOpts.Region)

	targets, err := targetFlagHelper(command.TargetOpts)
	if err != nil {
		return errors.Wrap(err, "failed to set targets")
	}

	fmt.Printf("Initialized with targets: %s\n", targets)
	fmt.Printf("Type 'exit' to exit. Use ctrl-c to abort running commands.\n\n")

	// (Parent) Context for the main thread and output channel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Catch sigterms to gracefully shut down
	var interrupts int
	abort := interruptHandler()

	// Reader for user prompt
	reader := bufio.NewReader(os.Stdin)

	for {
		cmd := userPrompt(reader)
		if cmd == "exit" {
			return nil
		}

		// Start command
		commandId, err := m.RunCommand(targets, cmd)
		if err != nil {
			return errors.Wrap(err, "failed to Run command")
		}
		out := make(chan *manager.CommandOutput)
		go m.GetCommandOutput(ctx, targets, commandId, out)

	Polling:
		for {
			select {
			case <-abort:
				interrupts++
				err := m.AbortCommand(targets, commandId)
				if err != nil {
					return errors.Wrap(err, "failed to abort command on sigterm")
				}
			case output, open := <-out:
				if output == nil && !open {
					break Polling
				}
				err := PrintCommandOutput(os.Stdout, output)
				if err != nil {
					return errors.Wrap(err, "failed to print output")
				}
			}
		}
	}
}
