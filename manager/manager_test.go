package manager_test

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var (
	ssmInstances = []*ssm.InstanceInformation{
		{
			InstanceId:       aws.String("i-00000000000000001"),
			PlatformName:     aws.String("Amazon Linux"),
			PlatformVersion:  aws.String("1.0"),
			IPAddress:        aws.String("10.0.0.1"),
			PingStatus:       aws.String("Online"),
			LastPingDateTime: aws.Time(time.Date(2018, time.January, 27, 13, 32, 0, 0, time.UTC)),
		},
		{
			InstanceId:       aws.String("i-00000000000000002"),
			PlatformName:     aws.String("Amazon Linux 2"),
			PlatformVersion:  aws.String("2.0"),
			IPAddress:        aws.String("10.0.0.100"),
			PingStatus:       aws.String("Online"),
			LastPingDateTime: aws.Time(time.Date(2018, time.January, 30, 13, 32, 0, 0, time.UTC)),
		},
	}

	ec2Instances = map[string]*ec2.Instance{
		"i-00000000000000001": {
			InstanceId: aws.String("i-00000000000000001"),
			ImageId:    aws.String("ami-db000001"),
			State:      &ec2.InstanceState{Name: aws.String("running")},
			Platform:   aws.String("Linux"),
			Tags: []*ec2.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String("instance 1"),
				},
			},
		},
		"i-00000000000000002": {
			InstanceId: aws.String("i-00000000000000002"),
			ImageId:    aws.String("ami-db000002"),
			State:      &ec2.InstanceState{Name: aws.String("running")},
			Platform:   aws.String("Linux"),
			Tags: []*ec2.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String("instance 2"),
				},
			},
		},
	}

	ssmDocumentIdentifiers = []*ssm.DocumentIdentifier{
		{
			Name:            aws.String("AWS-RunShellScript"),
			Owner:           aws.String("Amazon"),
			DocumentVersion: aws.String("1"),
			DocumentFormat:  aws.String("JSON"),
			DocumentType:    aws.String("Command"),
			SchemaVersion:   aws.String("1.2"),
			TargetType:      aws.String("Linux"),
		},
		{
			Name:            aws.String("Custom"),
			Owner:           aws.String("test-user"),
			DocumentVersion: aws.String("2"),
			DocumentFormat:  aws.String("YAML"),
			DocumentType:    aws.String("Command"),
			SchemaVersion:   aws.String("1.0"),
			TargetType:      aws.String("Windows"),
		},
	}

	ssmDocumentDescription = &ssm.DocumentDescription{
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

	outputInstances = []*manager.Instance{
		{
			InstanceID:       "i-00000000000000001",
			Name:             "instance 1",
			State:            "running",
			ImageID:          "ami-db000001",
			Platform:         "Linux",
			PlatformName:     "Amazon Linux",
			PlatformVersion:  "1.0",
			IPAddress:        "10.0.0.1",
			PingStatus:       "Online",
			LastPingDateTime: time.Date(2018, time.January, 27, 13, 32, 0, 0, time.UTC),
		},
		{
			InstanceID:       "i-00000000000000002",
			Name:             "instance 2",
			State:            "running",
			ImageID:          "ami-db000002",
			Platform:         "Linux",
			PlatformName:     "Amazon Linux 2",
			PlatformVersion:  "2.0",
			IPAddress:        "10.0.0.100",
			PingStatus:       "Online",
			LastPingDateTime: time.Date(2018, time.January, 30, 13, 32, 0, 0, time.UTC),
		},
	}

	outputDocumentIdentifiers = []*manager.DocumentIdentifier{
		{
			Name:            "AWS-RunShellScript",
			Owner:           "Amazon",
			DocumentVersion: "1",
			DocumentFormat:  "JSON",
			DocumentType:    "Command",
			SchemaVersion:   "1.2",
			TargetType:      "Linux",
		},
		{
			Name:            "Custom",
			Owner:           "test-user",
			DocumentVersion: "2",
			DocumentFormat:  "YAML",
			DocumentType:    "Command",
			SchemaVersion:   "1.0",
			TargetType:      "Windows",
		},
	}

	outputDocumentDescription = &manager.DocumentDescription{
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
)

func TestList(t *testing.T) {
	ssmMock := &manager.MockSSM{
		Error:         false,
		NextToken:     "",
		CommandStatus: "Success",
		CommandHistory: map[string]*struct {
			Command *ssm.Command
			Status  string
		}{},
		Instances: ssmInstances,
	}
	s3Mock := &manager.MockS3{
		Error: false,
	}
	ec2Mock := &manager.MockEC2{
		Error:     false,
		Instances: ec2Instances,
	}

	m := manager.NewTestManager(ssmMock, s3Mock, ec2Mock)

	t.Run("Get managed instances works", func(t *testing.T) {
		expected := outputInstances
		actual, err := m.ListInstances(50, nil)
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.ElementsMatch(t, expected, actual)
	})

	// t.Run("Limit number of instances works", func(t *testing.T) {
	// 	expected := outputInstances[:1]
	// 	actual, err := m.ListInstances(1, nil)
	// 	assert.Nil(t, err)
	// 	assert.NotNil(t, actual)
	// 	assert.ElementsMatch(t, expected, actual)
	// })
	//
	// t.Run("Pagination works", func(t *testing.T) {
	// 	ssmMock.NextToken = "next"
	// 	defer func() {
	// 		ssmMock.NextToken = ""
	// 	}()
	//
	// 	expected := outputInstances
	// 	actual, err := m.ListInstances(50, nil)
	// 	assert.Nil(t, err)
	// 	assert.NotNil(t, actual)
	// 	assert.ElementsMatch(t, expected, actual)
	// })

	t.Run("TagFilter works", func(t *testing.T) {
		expected := outputInstances[:1]
		actual, err := m.ListInstances(50, []*manager.TagFilter{
			{
				Key: "tag:Name",
				Values: []string{
					"1",
				},
			},
		})
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.ElementsMatch(t, expected, actual)
	})

	t.Run("Errors are propagated", func(t *testing.T) {
		ssmMock.Error = true
		defer func() {
			ssmMock.Error = false
		}()

		actual, err := m.ListInstances(50, nil)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "failed to describe instance information: expected")
		assert.Nil(t, actual)
	})
}

func TestListDocumentsCommand(t *testing.T) {
	ssmMock := &manager.MockSSM{
		Error:         false,
		NextToken:     "",
		CommandStatus: "Success",
		CommandHistory: map[string]*struct {
			Command *ssm.Command
			Status  string
		}{},
		Documents: ssmDocumentIdentifiers,
	}

	m := manager.NewTestManager(ssmMock, nil, nil)

	t.Run("List documents works", func(t *testing.T) {
		expected := outputDocumentIdentifiers
		actual, err := m.ListDocuments(50, nil)
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Limit number of documents works", func(t *testing.T) {
		expected := outputDocumentIdentifiers[:1]
		actual, err := m.ListDocuments(1, nil)
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Pagination works", func(t *testing.T) {
		ssmMock.NextToken = "next"
		defer func() {
			ssmMock.NextToken = ""
		}()

		expected := outputDocumentIdentifiers
		actual, err := m.ListDocuments(50, nil)
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Filter works", func(t *testing.T) {
		expected := outputDocumentIdentifiers[:1]
		actual, err := m.ListDocuments(50, []*ssm.DocumentFilter{
			{
				Key:   aws.String("Owner"),
				Value: aws.String("Amazon"),
			},
		})
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Errors are propagated", func(t *testing.T) {
		ssmMock.Error = true
		defer func() {
			ssmMock.Error = false
		}()

		actual, err := m.ListDocuments(50, nil)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "failed to list document: expected")
		assert.Nil(t, actual)
	})
}

func TestDescribeDocumentsCommand(t *testing.T) {
	ssmMock := &manager.MockSSM{
		Error:         false,
		NextToken:     "",
		CommandStatus: "Success",
		CommandHistory: map[string]*struct {
			Command *ssm.Command
			Status  string
		}{},
		DocumentDescription: ssmDocumentDescription,
	}

	m := manager.NewTestManager(ssmMock, nil, nil)

	t.Run("Describe documents works", func(t *testing.T) {
		expected := outputDocumentDescription
		actual, err := m.DescribeDocument("AWS-RunShellScript")
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Incorrect name failes", func(t *testing.T) {
		actual, err := m.DescribeDocument("Does-not-exist")
		assert.NotNil(t, err)
		assert.EqualError(t, err, "failed to describe document: expected")
		assert.Nil(t, actual)
	})
}

/*
func TestDescribeDocumentCommand(t *testing.T) {
}*/

func TestRunCommand(t *testing.T) {
	ssmMock := &manager.MockSSM{
		Error:         false,
		NextToken:     "",
		CommandStatus: "Success",
		CommandHistory: map[string]*struct {
			Command *ssm.Command
			Status  string
		}{},
		Instances: ssmInstances,
	}
	s3Mock := &manager.MockS3{
		Error: false,
	}
	ec2Mock := &manager.MockEC2{
		Error:     false,
		Instances: ec2Instances,
	}

	m := manager.NewTestManager(ssmMock, s3Mock, ec2Mock)

	var targets []string
	for _, instance := range ssmMock.Instances {
		targets = append(targets, aws.StringValue(instance.InstanceId))
	}

	t.Run("Run works", func(t *testing.T) {
		expected := "command-1"
		actual, err := m.RunCommand(targets, "AWS-RunShellScript", map[string]string{"commands": "ls -la"})
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Errors are propagated", func(t *testing.T) {
		ssmMock.Error = true
		defer func() {
			ssmMock.Error = false
		}()

		actual, err := m.RunCommand(targets, "AWS-RunShellScript", map[string]string{"commands": "ls -la"})
		assert.NotNil(t, err)
		assert.EqualError(t, err, "expected")
		assert.Equal(t, "", actual)
	})
}

func TestAbortCommand(t *testing.T) {
	ssmMock := &manager.MockSSM{
		Error:         false,
		NextToken:     "",
		CommandStatus: "Success",
		CommandHistory: map[string]*struct {
			Command *ssm.Command
			Status  string
		}{},
		Instances: ssmInstances,
	}
	s3Mock := &manager.MockS3{
		Error: false,
	}
	ec2Mock := &manager.MockEC2{
		Error:     false,
		Instances: ec2Instances,
	}

	m := manager.NewTestManager(ssmMock, s3Mock, ec2Mock)

	var targets []string
	for _, instance := range ssmMock.Instances {
		targets = append(targets, aws.StringValue(instance.InstanceId))
	}

	t.Run("Abort works", func(t *testing.T) {
		id, err := m.RunCommand(targets, "AWS-RunShellScript", map[string]string{"commands": "ls -la"})
		assert.Nil(t, err)
		err = m.AbortCommand(targets, id)
		assert.Nil(t, err)
	})

	t.Run("Invalid command id errors are propagated", func(t *testing.T) {
		_, err := m.RunCommand(targets, "AWS-RunShellScript", map[string]string{"commands": "ls -la"})
		assert.Nil(t, err)
		err = m.AbortCommand(targets, "invalid")
		assert.NotNil(t, err)
		assert.EqualError(t, err, "invalid commandId")
	})

	t.Run("Errors are propagated", func(t *testing.T) {
		ssmMock.Error = true
		defer func() {
			ssmMock.Error = false
		}()

		err := m.AbortCommand(targets, "na")
		assert.NotNil(t, err)
		assert.EqualError(t, err, "expected")
	})
}

func TestOutput(t *testing.T) {
	ssmMock := &manager.MockSSM{
		Error:         false,
		NextToken:     "",
		CommandStatus: "Success",
		CommandHistory: map[string]*struct {
			Command *ssm.Command
			Status  string
		}{},
		Instances: ssmInstances,
	}
	s3Mock := &manager.MockS3{
		Error: false,
	}
	ec2Mock := &manager.MockEC2{
		Error:     false,
		Instances: ec2Instances,
	}

	m := manager.NewTestManager(ssmMock, s3Mock, ec2Mock)

	var targets []string
	for _, instance := range ssmMock.Instances {
		targets = append(targets, aws.StringValue(instance.InstanceId))
	}

	t.Run("Get output works with standard out", func(t *testing.T) {
		id, err := m.RunCommand(targets, "AWS-RunShellScript", map[string]string{"commands": "ls -la"})
		assert.Nil(t, err)

		ctx := context.Background()
		out := make(chan *manager.CommandOutput)
		go m.GetCommandOutput(ctx, targets, id, out)

		var actual []string

		for o := range out {
			assert.Nil(t, o.Error)
			assert.Equal(t, "Success", o.Status)
			assert.Equal(t, "example standard output", o.Output)
			actual = append(actual, o.InstanceID)
		}
		assert.Equal(t, len(targets), len(actual))
	})

	t.Run("Get output works with standard error", func(t *testing.T) {
		ssmMock.CommandStatus = "Failed"
		defer func() {
			ssmMock.CommandStatus = "Success"
		}()

		id, err := m.RunCommand(targets, "AWS-RunShellScript", map[string]string{"commands": "ls -la"})
		assert.Nil(t, err)

		ctx := context.Background()
		out := make(chan *manager.CommandOutput)
		go m.GetCommandOutput(ctx, targets, id, out)

		for o := range out {
			assert.Nil(t, o.Error)
			assert.Equal(t, "Failed", o.Status)
			assert.Equal(t, "example standard error", o.Output)
		}
	})

	t.Run("Get output is aborted if the context is done", func(t *testing.T) {
		id, err := m.RunCommand(targets, "AWS-RunShellScript", map[string]string{"commands": "ls -la"})
		assert.Nil(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		out := make(chan *manager.CommandOutput)

		cancel()
		go m.GetCommandOutput(ctx, targets, id, out)

		var actual []string

		for o := range out {
			actual = append(actual, o.InstanceID)
		}
		assert.Equal(t, 0, len(actual))
	})
}
