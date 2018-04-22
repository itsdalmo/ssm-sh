package command

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
	"os"
	"strings"
)

// ListDocumentsCommand contains all arguments for list-documents command
type ListDocumentsCommand struct {
	Filters []*documentFilter `short:"f" long:"filter" description:"Filter the produced list by property (Name, Owner, DocumentType, PlatformTypes)"`
	Limit   int64             `short:"l" long:"limit" description:"Limit the number of instances printed" default:"50"`
}

type documentFilter ssm.DocumentFilter

// Execute list-documents command
func (command *ListDocumentsCommand) Execute([]string) error {
	sess, err := newSession()
	if err != nil {
		return errors.Wrap(err, "failed to create new session")
	}
	m := manager.NewManager(sess, Command.AwsOpts.Region)

	var filters []*ssm.DocumentFilter
	for _, filter := range command.Filters {
		filters = append(filters, &ssm.DocumentFilter{
			Key:   filter.Key,
			Value: filter.Value,
		})
	}

	documents, err := m.ListDocuments(command.Limit, filters)
	if err != nil {
		return errors.Wrap(err, "failed to list documents")
	}

	if err := PrintDocuments(os.Stdout, documents); err != nil {
		return errors.Wrap(err, "failed to print documents")
	}

	return nil
}

func (d *documentFilter) UnmarshalFlag(input string) error {
	parts := strings.Split(input, "=")
	if len(parts) != 2 {
		return errors.New("expected a key and a value separated by =")
	}

	d.Key = aws.String(parts[0])
	d.Value = aws.String(parts[1])

	return nil
}
