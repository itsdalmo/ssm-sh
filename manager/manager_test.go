package manager_test

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"strings"
	"sync"
	"testing"
	"time"
)

func generateInstances() []*ssm.InstanceInformation {
	var out []*ssm.InstanceInformation

	// Generate the desired output
	for i := 1; i <= 10; i++ {
		instance := &ssm.InstanceInformation{
			InstanceId:       aws.String(fmt.Sprintf("i-%017d", i)),
			PlatformName:     aws.String("Amazon Linux"),
			PlatformVersion:  aws.String("2.0"),
			IPAddress:        aws.String(fmt.Sprintf("10.0.0.%d", i)),
			LastPingDateTime: aws.Time(time.Date(2018, time.January, 27, 0, 0, 0, 0, time.UTC)),
		}
		out = append(out, instance)
	}
	return out
}

type MockS3 struct {
	s3iface.S3API
	ShouldError bool
}

func (mock *MockS3) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	if mock.ShouldError {
		return nil, errors.New("expected")
	}
	if input.Bucket == nil {
		return nil, errors.New("Missing Bucket")
	}
	if input.Key == nil {
		return nil, errors.New("Missing Key")
	}

	return &s3.GetObjectOutput{
		Body: ioutil.NopCloser(strings.NewReader("example s3 output")),
	}, nil
}

type MockSSM struct {
	ssmiface.SSMAPI
	ShouldError    bool
	CommandStatus  string
	CommandHistory []string
	NextToken      string
	OutputDelay    int
	Aborted        bool
	l              sync.Mutex
}

func (mock *MockSSM) DescribeInstanceInformation(input *ssm.DescribeInstanceInformationInput) (*ssm.DescribeInstanceInformationOutput, error) {
	if mock.ShouldError {
		return nil, errors.New("expected")
	}

	count := int(aws.Int64Value(input.MaxResults))
	instances := generateInstances()[:count]
	var output *ssm.DescribeInstanceInformationOutput

	// To test NextToken we generate N instances and send half on first request, rest on 2nd.
	if mock.NextToken != "" {
		if input.NextToken == nil {
			output = &ssm.DescribeInstanceInformationOutput{
				InstanceInformationList: instances[:count/2],
				NextToken:               aws.String(mock.NextToken),
			}
		} else if aws.StringValue(input.NextToken) == mock.NextToken {
			output = &ssm.DescribeInstanceInformationOutput{
				InstanceInformationList: instances[count/2:],
				NextToken:               nil,
			}
		} else {
			return nil, errors.New("Wrong token")
		}
	} else {
		output = &ssm.DescribeInstanceInformationOutput{
			InstanceInformationList: instances,
			NextToken:               nil,
		}
	}

	return output, nil
}

func (mock *MockSSM) SendCommand(input *ssm.SendCommandInput) (*ssm.SendCommandOutput, error) {
	if mock.ShouldError {
		return nil, errors.New("expected")
	}

	// Validate required input and intended behavior.
	if input.DocumentName == nil {
		return nil, errors.New("Missing comment")
	}

	if input.InstanceIds == nil || len(input.InstanceIds) == 0 {
		return nil, errors.New("Missing InstanceIds")
	}

	if input.Parameters == nil {
		return nil, errors.New("Missing parameters")
	}

	_, ok := input.Parameters["commands"]
	if !ok {
		return nil, errors.New("Missing commands in Parameters")
	}

	// Generate a new command and add it to command history
	command := fmt.Sprintf("command %d", len(mock.CommandHistory)+1)
	mock.CommandHistory = append(mock.CommandHistory, command)

	return &ssm.SendCommandOutput{
		Command: &ssm.Command{
			CommandId:          aws.String(command),
			Comment:            input.Comment,
			DocumentName:       input.DocumentName,
			InstanceIds:        input.InstanceIds,
			OutputS3BucketName: input.OutputS3BucketName,
			OutputS3KeyPrefix:  input.OutputS3KeyPrefix,
		},
	}, nil
}

func (mock *MockSSM) CancelCommand(input *ssm.CancelCommandInput) (*ssm.CancelCommandOutput, error) {
	if mock.ShouldError {
		return nil, errors.New("expected")
	}

	if input.CommandId == nil {
		return nil, errors.New("Missing CommandId")
	}

	if input.InstanceIds == nil || len(input.InstanceIds) == 0 {
		return nil, errors.New("Missing InstanceIds")
	}

	// Make sure a command by this ID has been run before
	for _, id := range mock.CommandHistory {
		if aws.StringValue(input.CommandId) == id {
			mock.l.Lock()
			mock.Aborted = true
			defer mock.l.Unlock()
			return &ssm.CancelCommandOutput{}, nil
		}
	}

	// Only reachable if ID does not exist
	return nil, errors.New(ssm.ErrCodeInvalidCommandId)
}

func (mock *MockSSM) GetCommandInvocation(input *ssm.GetCommandInvocationInput) (*ssm.GetCommandInvocationOutput, error) {
	if mock.ShouldError {
		return nil, errors.New("expected")
	}

	if input.CommandId == nil {
		return nil, errors.New("Missing CommandId")
	}

	if input.InstanceId == nil {
		return nil, errors.New("Missing InstanceId")
	}

	if mock.OutputDelay > 0 {
		time.Sleep(time.Duration(mock.OutputDelay) * time.Millisecond)
	}

	var status string
	mock.l.Lock()
	defer mock.l.Unlock()
	if mock.Aborted {
		status = "Cancelled"
	} else {
		status = mock.CommandStatus
	}

	return &ssm.GetCommandInvocationOutput{
		InstanceId:            input.InstanceId,
		StatusDetails:         aws.String(status),
		StandardOutputContent: aws.String("example standard output"),
		StandardErrorContent:  aws.String("example standard error"),
	}, nil
}

func TestGetInstances(t *testing.T) {
	ssmMock := &MockSSM{
		ShouldError:    false,
		CommandStatus:  "Success",
		CommandHistory: make([]string, 0),
		NextToken:      "",
	}
	s3Mock := &MockS3{ShouldError: false}
	m := manager.NewTestManager(ssmMock, s3Mock)

	t.Run("Get managed instances works", func(t *testing.T) {
		expected := generateInstances()
		actual, err := m.GetInstances(10)
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Limit number of instances works", func(t *testing.T) {
		expected := generateInstances()[:5]
		actual, err := m.GetInstances(5)
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Pagination works", func(t *testing.T) {
		ssmMock.NextToken = "next"
		defer func() {
			ssmMock.NextToken = ""
		}()

		expected := generateInstances()
		actual, err := m.GetInstances(10)
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Errors are propagated", func(t *testing.T) {
		ssmMock.ShouldError = true
		defer func() {
			ssmMock.ShouldError = false
		}()

		actual, err := m.GetInstances(10)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "expected")
		assert.Nil(t, actual)
	})
}

func TestList(t *testing.T) {
	ssmMock := &MockSSM{
		ShouldError:    false,
		CommandStatus:  "Success",
		CommandHistory: make([]string, 0),
		NextToken:      "",
	}
	s3Mock := &MockS3{ShouldError: false}
	m := manager.NewTestManager(ssmMock, s3Mock)

	t.Run("List works and can be limited", func(t *testing.T) {
		b := new(bytes.Buffer)
		expected := strings.TrimSpace(`
Instance ID         | Platform     | Version | IP       | Last pinged
i-00000000000000001 | Amazon Linux | 2.0     | 10.0.0.1 | 2018-01-27
i-00000000000000002 | Amazon Linux | 2.0     | 10.0.0.2 | 2018-01-27
i-00000000000000003 | Amazon Linux | 2.0     | 10.0.0.3 | 2018-01-27
i-00000000000000004 | Amazon Linux | 2.0     | 10.0.0.4 | 2018-01-27
i-00000000000000005 | Amazon Linux | 2.0     | 10.0.0.5 | 2018-01-27
`)
		err := m.List(b, 5)
		actual := strings.TrimSpace(b.String())
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Errors are propagated", func(t *testing.T) {
		b := new(bytes.Buffer)
		ssmMock.ShouldError = true
		defer func() {
			ssmMock.ShouldError = false
		}()

		err := m.List(b, 5)
		assert.NotNil(t, err)
		assert.EqualError(t, err, "expected")
	})
}

func TestRun(t *testing.T) {
	ssmMock := &MockSSM{
		ShouldError:    false,
		CommandStatus:  "Success",
		CommandHistory: make([]string, 0),
		NextToken:      "",
	}
	s3Mock := &MockS3{ShouldError: false}
	m := manager.NewTestManager(ssmMock, s3Mock)

	instances := generateInstances()
	var targets []string
	for _, instance := range instances {
		targets = append(targets, aws.StringValue(instance.InstanceId))
	}

	t.Run("Run works", func(t *testing.T) {
		expected := "command 1"
		actual, err := m.Run(targets, "ls -la")
		assert.Nil(t, err)
		assert.NotNil(t, actual)
		assert.Equal(t, expected, actual)
	})

	t.Run("Errors are propagated", func(t *testing.T) {
		ssmMock.ShouldError = true
		defer func() {
			ssmMock.ShouldError = false
		}()

		actual, err := m.Run(targets, "ls -la")
		assert.NotNil(t, err)
		assert.EqualError(t, err, "expected")
		assert.Equal(t, "", actual)
	})
}

func TestAbort(t *testing.T) {
	ssmMock := &MockSSM{
		ShouldError:    false,
		CommandStatus:  "Success",
		CommandHistory: make([]string, 0),
		NextToken:      "",
	}
	s3Mock := &MockS3{ShouldError: false}
	m := manager.NewTestManager(ssmMock, s3Mock)

	instances := generateInstances()
	var targets []string
	for _, instance := range instances {
		targets = append(targets, aws.StringValue(instance.InstanceId))
	}

	t.Run("Abort works", func(t *testing.T) {
		id, err := m.Run(targets, "ls -la")
		assert.Nil(t, err)
		err = m.Abort(targets, id)
		assert.Nil(t, err)
	})

	t.Run("Invalid command id errors are propagated", func(t *testing.T) {
		_, err := m.Run(targets, "ls -la")
		assert.Nil(t, err)
		err = m.Abort(targets, "invalid")
		assert.NotNil(t, err)
		assert.EqualError(t, err, ssm.ErrCodeInvalidCommandId)
	})

	t.Run("Errors are propagated", func(t *testing.T) {
		ssmMock.ShouldError = true
		defer func() {
			ssmMock.ShouldError = false
		}()

		err := m.Abort(targets, "na")
		assert.NotNil(t, err)
		assert.EqualError(t, err, "expected")
	})
}

func TestGetOutput(t *testing.T) {
	ssmMock := &MockSSM{
		ShouldError:    false,
		CommandStatus:  "Success",
		CommandHistory: make([]string, 0),
		NextToken:      "",
	}
	s3Mock := &MockS3{ShouldError: false}
	m := manager.NewTestManager(ssmMock, s3Mock)

	instances := generateInstances()
	var targets []string
	for _, instance := range instances {
		targets = append(targets, aws.StringValue(instance.InstanceId))
	}

	t.Run("GetOutput works", func(t *testing.T) {
		id, err := m.Run(targets, "ls -la")
		assert.Nil(t, err)

		out := make(chan manager.CommandOutput)
		abort := make(chan bool)
		defer close(abort)
		go m.GetOutput(targets, id, out, abort)

		var actual []string

		for o := range out {
			assert.Nil(t, o.Error)
			assert.Equal(t, "Success", o.Status)
			assert.Equal(t, "example standard output", o.Output)
			actual = append(actual, o.InstanceId)
		}
		assert.Equal(t, len(targets), len(actual))
	})

	t.Run("GetOutput works with delay", func(t *testing.T) {
		ssmMock.OutputDelay = 50 // Milliseconds
		defer func() {
			ssmMock.OutputDelay = 0
		}()

		id, err := m.Run(targets, "ls -la")
		assert.Nil(t, err)

		out := make(chan manager.CommandOutput)
		abort := make(chan bool)
		defer close(abort)
		go m.GetOutput(targets[:1], id, out, abort)

		var actual []string

		for o := range out {
			assert.Nil(t, o.Error)
			assert.Equal(t, "Success", o.Status)
			assert.Equal(t, "example standard output", o.Output)
			actual = append(actual, o.InstanceId)
		}
		assert.Equal(t, 1, len(actual))
	})

	t.Run("GetOutput works with standard error", func(t *testing.T) {
		ssmMock.CommandStatus = "Failed"
		defer func() {
			ssmMock.CommandStatus = "Success"
		}()

		id, err := m.Run(targets, "ls -la")
		assert.Nil(t, err)

		out := make(chan manager.CommandOutput)
		abort := make(chan bool)
		defer close(abort)
		go m.GetOutput(targets, id, out, abort)

		for o := range out {
			assert.Nil(t, o.Error)
			assert.Equal(t, "Failed", o.Status)
			assert.Equal(t, "example standard error", o.Output)
		}
	})
}

func TestOutput(t *testing.T) {
	ssmMock := &MockSSM{
		ShouldError:    false,
		CommandStatus:  "Success",
		CommandHistory: make([]string, 0),
		NextToken:      "",
	}
	s3Mock := &MockS3{ShouldError: false}
	m := manager.NewTestManager(ssmMock, s3Mock)
	targets := []string{"i-00000000000000001"}

	t.Run("Output works", func(t *testing.T) {
		id, err := m.Run(targets, "ls -la")
		assert.Nil(t, err)

		b := new(bytes.Buffer)
		abort := make(chan bool)
		defer close(abort)
		m.Output(b, targets, id, abort)

		actual := strings.TrimSpace(b.String())

		assert.Contains(t, actual, "i-00000000000000001")
		assert.Contains(t, actual, "Success")
		assert.Contains(t, actual, "example standard output")
	})

	t.Run("Output is interruptable", func(t *testing.T) {
		id, err := m.Run(targets, "ls -la")
		assert.Nil(t, err)

		b := new(bytes.Buffer)
		abort := make(chan bool, 1)
		defer close(abort)
		abort <- true

		m.Output(b, targets, id, abort)

		actual := strings.TrimSpace(b.String())

		assert.Contains(t, actual, "i-00000000000000001")
		assert.Contains(t, actual, "Cancelled")
		assert.Contains(t, actual, "Command was aborted")
	})
}
