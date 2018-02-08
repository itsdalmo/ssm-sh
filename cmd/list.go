package cmd

import (
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
	"os"
)

type ListCommand struct {
	Limit int64 `short:"l" long:"limit" description:"Limit the number of instances printed" default:"50"`
}

func (command *ListCommand) Execute([]string) error {
	sess, err := newSession()
	if err != nil {
		return errors.Wrap(err, "failed to create new session")
	}
	m := manager.NewManager(sess, Command.AwsOpts.Region)

	instances, err := m.ListInstances(command.Limit)
	if err != nil {
		return errors.Wrap(err, "failed to list instances")
	}

	if err := PrintInstances(os.Stdout, instances); err != nil {
		return errors.Wrap(err, "failed to print instances")
	}

	return nil
}
