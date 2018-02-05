package command

import (
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
	"os"
)

type ListCommand struct {
	Limit int `short:"l" long:"limit" description:"Limit the number of instances printed" default:"50"`
}

func (command *ListCommand) Execute([]string) error {
	sess, err := newSession()
	if err != nil {
		return errors.Wrap(err, "Failed to create new AWS session")
	}

	svc := manager.NewManager(sess, Cmd.AwsOpts.Region, 0, 0)
	if err := svc.List(os.Stdout, command.Limit); err != nil {
		return errors.Wrap(err, "Failed to list instances")
	}

	return nil
}
