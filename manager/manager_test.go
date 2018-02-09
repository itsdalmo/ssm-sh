package manager_test

import (
	"context"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var (
	inputInstances = []*ssm.InstanceInformation{
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

	outputInstances = []*manager.Instance{
		{
			InstanceID:       "i-00000000000000001",
			PlatformName:     "Amazon Linux",
			PlatformVersion:  "1.0",
			IPAddress:        "10.0.0.1",
			PingStatus:       "Online",
			LastPingDateTime: time.Date(2018, time.January, 27, 13, 32, 0, 0, time.UTC),
		},
		{
			InstanceID:       "i-00000000000000002",
			PlatformName:     "Amazon Linux 2",
			PlatformVersion:  "2.0",
			IPAddress:        "10.0.0.100",
			PingStatus:       "Online",
			LastPingDateTime: time.Date(2018, time.January, 30, 13, 32, 0, 0, time.UTC),
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
		Instances: inputInstances,
	}
	s3Mock := &manager.MockS3{
		Error: false,
	}

	m := manager.NewTestManager(ssmMock, s3Mock)

	t.Run("Get managed instances works", func(t *testing.T) {
		expected := outputInstances
		actual, err := m.ListInstances(50)
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Limit number of instances works", func(t *testing.T) {
		expected := outputInstances[:1]
		actual, err := m.ListInstances(1)
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Pagination works", func(t *testing.T) {
		ssmMock.NextToken = "next"
		defer func() {
			ssmMock.NextToken = ""
		}()

		expected := outputInstances
		actual, err := m.ListInstances(50)
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Errors are propagated", func(t *testing.T) {
		ssmMock.Error = true
		defer func() {
			ssmMock.Error = false
		}()

		actual, err := m.ListInstances(50)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "failed to describe instance information: expected")
		assert.Nil(t, actual)
	})
}

func TestRunCommand(t *testing.T) {
	ssmMock := &manager.MockSSM{
		Error:         false,
		NextToken:     "",
		CommandStatus: "Success",
		CommandHistory: map[string]*struct {
			Command *ssm.Command
			Status  string
		}{},
		Instances: inputInstances,
	}
	s3Mock := &manager.MockS3{
		Error: false,
	}

	m := manager.NewTestManager(ssmMock, s3Mock)

	var targets []string
	for _, instance := range ssmMock.Instances {
		targets = append(targets, aws.StringValue(instance.InstanceId))
	}

	t.Run("Run works", func(t *testing.T) {
		expected := "command-1"
		actual, err := m.RunCommand(targets, "ls -la")
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Errors are propagated", func(t *testing.T) {
		ssmMock.Error = true
		defer func() {
			ssmMock.Error = false
		}()

		actual, err := m.RunCommand(targets, "ls -la")
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
		Instances: inputInstances,
	}
	s3Mock := &manager.MockS3{
		Error: false,
	}

	m := manager.NewTestManager(ssmMock, s3Mock)

	var targets []string
	for _, instance := range ssmMock.Instances {
		targets = append(targets, aws.StringValue(instance.InstanceId))
	}

	t.Run("Abort works", func(t *testing.T) {
		id, err := m.RunCommand(targets, "ls -la")
		assert.Nil(t, err)
		err = m.AbortCommand(targets, id)
		assert.Nil(t, err)
	})

	t.Run("Invalid command id errors are propagated", func(t *testing.T) {
		_, err := m.RunCommand(targets, "ls -la")
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
		Instances: inputInstances,
	}
	s3Mock := &manager.MockS3{
		Error: false,
	}

	m := manager.NewTestManager(ssmMock, s3Mock)

	var targets []string
	for _, instance := range ssmMock.Instances {
		targets = append(targets, aws.StringValue(instance.InstanceId))
	}

	t.Run("Get output works with standard out", func(t *testing.T) {
		id, err := m.RunCommand(targets, "ls -la")
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

		id, err := m.RunCommand(targets, "ls -la")
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
		id, err := m.RunCommand(targets, "ls -la")
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
