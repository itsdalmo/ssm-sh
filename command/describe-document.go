package command

import (
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
	"os"
)

// DescribeDocumentCommand contains all arguments for describe-document command
type DescribeDocumentCommand struct {
	Name string `short:"n" long:"name" description:"Name of document in ssm."`
}

// Execute describe-documents command
func (command *DescribeDocumentCommand) Execute(args []string) error {
	if command.Name == "" {
		return errors.New("No document name set")
	}

	sess, err := newSession()
	if err != nil {
		return errors.Wrap(err, "failed to create new aws session")
	}

	m := manager.NewManager(sess, Command.AwsOpts.Region)

	document, err := m.DescribeDocument(command.Name)
	if err != nil {
		return errors.Wrap(err, "failed to describe document")
	}

	if err := PrintDocumentDescription(os.Stdout, document); err != nil {
		return errors.Wrap(err, "failed to print document")
	}
	return nil
}
