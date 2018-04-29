package manager_test

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDocumentDescription(t *testing.T) {
	ssmDocumentDescriptionWithoutParameters := &ssm.DocumentDescription{
		Name:            aws.String("AWS-RunShellScript"),
		Description:     aws.String("Run a shell script or specify the commands to run."),
		Owner:           aws.String("Amazon"),
		DocumentVersion: aws.String("1"),
		DocumentFormat:  aws.String("JSON"),
		DocumentType:    aws.String("Command"),
		SchemaVersion:   aws.String("1.2"),
		TargetType:      aws.String("Linux"),
	}

	ssmDocumentDescriptionWithParameters := &ssm.DocumentDescription{
		Name:            aws.String("AWS-RunShellScript"),
		Description:     aws.String("Run a shell script or specify the commands to run."),
		Owner:           aws.String("Amazon"),
		DocumentVersion: aws.String("1"),
		DocumentFormat:  aws.String("JSON"),
		DocumentType:    aws.String("Command"),
		SchemaVersion:   aws.String("1.2"),
		TargetType:      aws.String("Linux"),
		Parameters: []*ssm.DocumentParameter{
			{
				Name:         aws.String("commands"),
				Description:  aws.String("Specify a shell script or a command to run"),
				DefaultValue: aws.String(""),
				Type:         aws.String("StringList"),
			},
			{
				Name:         aws.String("executionTimeout"),
				Description:  aws.String("The time in seconds for a command to complete"),
				DefaultValue: aws.String("3600"),
				Type:         aws.String("String"),
			},
		},
	}

	outputWithoutParameters := &manager.DocumentDescription{
		Name:            "AWS-RunShellScript",
		Description:     "Run a shell script or specify the commands to run.",
		Owner:           "Amazon",
		DocumentVersion: "1",
		DocumentFormat:  "JSON",
		DocumentType:    "Command",
		SchemaVersion:   "1.2",
		TargetType:      "Linux",
	}

	outputWithParameters := &manager.DocumentDescription{
		Name:            "AWS-RunShellScript",
		Description:     "Run a shell script or specify the commands to run.",
		Owner:           "Amazon",
		DocumentVersion: "1",
		DocumentFormat:  "JSON",
		DocumentType:    "Command",
		SchemaVersion:   "1.2",
		TargetType:      "Linux",
		Parameters: []*manager.DocumentParameter{
			{
				Name:         "commands",
				Description:  "Specify a shell script or a command to run",
				DefaultValue: "",
				Type:         "StringList",
			},
			{
				Name:         "executionTimeout",
				Description:  "The time in seconds for a command to complete",
				DefaultValue: "3600",
				Type:         "String",
			},
		},
	}

	t.Run("NewDocumentDescription works", func(t *testing.T) {
		expected := outputWithoutParameters
		actual := manager.NewDocumentDescription(ssmDocumentDescriptionWithoutParameters)
		assert.Equal(t, expected, actual)
	})

	t.Run("Instance TabString works", func(t *testing.T) {
		expected := "AWS-RunShellScript\t|\tRun a shell script or specify the commands to run.\t|\tAmazon\t|\t1\t|\tJSON\t|\tCommand\t|\t1.2\t|\tLinux"
		actual := outputWithoutParameters.TabString()
		assert.Equal(t, expected, actual)
	})

	t.Run("NewDocumentDescription works with parameters", func(t *testing.T) {
		expected := outputWithParameters
		actual := manager.NewDocumentDescription(ssmDocumentDescriptionWithParameters)
		assert.Equal(t, expected, actual)
	})

	t.Run("Instance ParametersTabString works", func(t *testing.T) {
		expected := "commands\t|\tStringList\t|\t\t|\tSpecify a shell script or a command to run\nexecutionTimeout\t|\tString\t|\t3600\t|\tThe time in seconds for a command to complete"
		actual := outputWithParameters.ParametersTabString()
		assert.Equal(t, expected, actual)
	})
}
