package manager_test

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDocumentIdentifier(t *testing.T) {
	ssmDocumentIdentifier := &ssm.DocumentIdentifier{
		Name:            aws.String("AWS-RunShellScript"),
		Owner:           aws.String("Amazon"),
		DocumentVersion: aws.String("1"),
		DocumentFormat:  aws.String("JSON"),
		DocumentType:    aws.String("Command"),
		SchemaVersion:   aws.String("1.2"),
		TargetType:      aws.String("Linux"),
	}

	output := &manager.DocumentIdentifier{
		Name:            "AWS-RunShellScript",
		Owner:           "Amazon",
		DocumentVersion: "1",
		DocumentFormat:  "JSON",
		DocumentType:    "Command",
		SchemaVersion:   "1.2",
		TargetType:      "Linux",
	}

	t.Run("NewDocumentIdentifier works", func(t *testing.T) {
		expected := output
		actual := manager.NewDocumentIdentifier(ssmDocumentIdentifier)
		assert.Equal(t, expected, actual)
	})

	t.Run("Instance TabString works", func(t *testing.T) {
		expected := "AWS-RunShellScript\t|\tAmazon\t|\t1\t|\tJSON\t|\tCommand\t|\t1.2\t|\tLinux"
		actual := output.TabString()
		assert.Equal(t, expected, actual)
	})
}
