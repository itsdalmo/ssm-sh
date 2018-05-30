package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
)

type ShellCommand struct {
	SSMOpts    SSMOptions `group:"SSM options"`
	TargetOpts TargetOptions
}

func (command *ShellCommand) Execute([]string) error {
	var shellDocument = "AWS-RunShellScript"

	sess, err := newSession()

	if err != nil {
		return errors.Wrap(err, "failed to create new aws session")
	}

	opts, err := command.SSMOpts.Parse()
	if err != nil {
		return err
	}
	m := manager.NewManager(sess, Command.AwsOpts.Region, *opts)
	targets, err := setTargets(command.TargetOpts)
	if err != nil {
		return errors.Wrap(err, "failed to set targets")
	}
	fmt.Printf("Type 'exit' to exit. Use ctrl-c to abort running commands.\n\n")

	var filters []*manager.TagFilter
	filters = append(filters, &manager.TagFilter{
		Key:    "platform",
		Values: []string{"windows"},
	})

	windowsTargets, err := m.FilterInstances(targets, filters)
	if len(targets) != len(windowsTargets) {
		errors.New("Targets: Cannot mix WIndows and Linux targets")
	}
	if len(windowsTargets) > 0 {
		fmt.Printf("Windows Targets: %d \n\n", len(windowsTargets))
		shellDocument = "AWS-RunPowerShellScript"
	}

	// (Parent) Context for the main thread and output channel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Catch sigterms to gracefully shut down
	var interrupts int
	abort := interruptHandler()

	// Configure readline
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[31m»\033[0m ",
		HistoryFile:     "/tmp/ssh-sh.tmp",
		InterruptPrompt: "^C",
		EOFPrompt:       "^D",
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	for {
		cmd, err := rl.Readline()

		if err == readline.ErrInterrupt {
			continue
		} else if err == io.EOF {
			return nil
		}

		cmd = strings.TrimSpace(cmd)
		if len(cmd) == 0 {
			continue
		} else if cmd == "exit" {
			return nil
		}

		// Start command
		commandID, err := m.RunCommand(targets, shellDocument, map[string]string{"commands": cmd})
		if err != nil {
			return errors.Wrap(err, "failed to Run command")
		}
		out := make(chan *manager.CommandOutput)
		go m.GetCommandOutput(ctx, targets, commandID, out)

	Polling:
		for {
			select {
			case <-abort:
				interrupts++
				err := m.AbortCommand(targets, commandID)
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
