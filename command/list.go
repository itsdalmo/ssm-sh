package command

import (
	"encoding/json"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
)

type ListCommand struct {
	Limit  int64  `short:"l" long:"limit" description:"Limit the number of instances printed" default:"50"`
	Output string `short:"o" long:"output" description:"Path to a file where the list of instances will be written as JSON."`
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

	if command.Output != "" {
		j, err := json.MarshalIndent(instances, "", "    ")
		if err != nil {
			return errors.Wrap(err, "failed to marshal instances")
		}
		if err := ioutil.WriteFile(command.Output, j, 0644); err != nil {
			return errors.Wrap(err, "failed to write output")
		}
	}

	return nil
}
