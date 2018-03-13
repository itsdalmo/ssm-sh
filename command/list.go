package command

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"

	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
)

type ListCommand struct {
	Tags   []*tag `short:"f" long:"filter" description:"Filter the produced list by tag (key=value,..)"`
	Limit  int64  `short:"l" long:"limit" description:"Limit the number of instances printed" default:"50"`
	Output string `short:"o" long:"output" description:"Path to a file where the list of instances will be written as JSON."`
}

func (command *ListCommand) Execute([]string) error {
	sess, err := newSession()
	if err != nil {
		return errors.Wrap(err, "failed to create new session")
	}
	m := manager.NewManager(sess, Command.AwsOpts.Region, manager.Opts{})

	var filters []*manager.TagFilter
	for _, tag := range command.Tags {
		filters = append(filters, &manager.TagFilter{
			Key:    tag.Key,
			Values: tag.Values,
		})
	}
	instances, err := m.ListInstances(command.Limit, filters)
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

type tag manager.TagFilter

func (t *tag) UnmarshalFlag(value string) error {
	parts := strings.Split(value, "=")
	if len(parts) != 2 {
		return errors.New("expected a key and a value separated by =")
	}

	values := strings.Split(parts[1], ",")
	if len(values) < 1 {
		return errors.New("expected one or more values separated by ,")
	}

	t.Key = parts[0]
	t.Values = values

	return nil
}
