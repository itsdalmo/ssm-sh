package manager

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/pkg/errors"
	"io"
	"sync"
	"time"
)

// Manager handles the clients interfacing with AWS.
type Manager struct {
	ssmClient ssmiface.SSMAPI
	s3Client  s3iface.S3API
	region    string
}

// NewManager creates a new Manager from an AWS session and region.
func NewManager(sess *session.Session, region string) *Manager {
	config := &aws.Config{Region: aws.String(region)}
	return &Manager{
		ssmClient: ssm.New(sess, config),
		s3Client:  s3.New(sess, config),
		region:    region,
	}
}

// NewTestManager creates a new manager for testing purposes.
func NewTestManager(ssm ssmiface.SSMAPI, s3 s3iface.S3API) *Manager {
	return &Manager{
		ssmClient: ssm,
		s3Client:  s3,
		region:    "eu-west-1",
	}
}

// ListInstances fetches a list of instances managed by SSM. Paginates until all responses have been collected.
func (m *Manager) ListInstances(limit int64) ([]*Instance, error) {
	var out []*Instance

	input := &ssm.DescribeInstanceInformationInput{
		MaxResults: &limit,
	}

	for {
		response, err := m.ssmClient.DescribeInstanceInformation(input)
		if err != nil {
			return nil, errors.Wrap(err, "failed to describe instance information")
		}
		for _, instance := range response.InstanceInformationList {
			out = append(out, NewInstance(instance))
		}
		if response.NextToken == nil {
			break
		}
		input.NextToken = response.NextToken
	}

	return out, nil
}

// RunCommand on the given instance ids.
func (m *Manager) RunCommand(instanceIds []string, command string) (string, error) {
	input := &ssm.SendCommandInput{
		InstanceIds:  aws.StringSlice(instanceIds),
		DocumentName: aws.String("AWS-RunShellScript"),
		Comment:      aws.String("Interactive command."),
		Parameters:   map[string][]*string{"commands": {aws.String(command)}},
	}

	res, err := m.ssmClient.SendCommand(input)
	if err != nil {
		return "", err
	}

	return aws.StringValue(res.Command.CommandId), nil
}

// AbortCommand command on the given instance ids.
func (m *Manager) AbortCommand(instanceIds []string, commandID string) error {
	_, err := m.ssmClient.CancelCommand(&ssm.CancelCommandInput{
		CommandId:   aws.String(commandID),
		InstanceIds: aws.StringSlice(instanceIds),
	})
	if err != nil {
		return err
	}
	return nil
}

// CommandOutput is the return type transmitted over a channel when fetching output.
type CommandOutput struct {
	InstanceID string
	Status     string
	Output     string
	Error      error
}

// GetCommandOutput fetches the results from a command invocation for all specified instanceIds and
// closes the receiving channel before exiting.
func (m *Manager) GetCommandOutput(ctx context.Context, instanceIds []string, commandID string, out chan<- *CommandOutput) {
	defer close(out)
	var wg sync.WaitGroup

	for _, id := range instanceIds {
		wg.Add(1)
		go m.pollInstanceOutput(ctx, id, commandID, out, &wg)
	}

	wg.Wait()
	return
}

// Fetch output from a command invocation on an instance.
func (m *Manager) pollInstanceOutput(ctx context.Context, instanceID string, commandID string, c chan<- *CommandOutput, wg *sync.WaitGroup) {
	defer wg.Done()
	retry := time.NewTicker(time.Millisecond * time.Duration(500))

	for {
		select {
		case <-ctx.Done():
			// Main thread is no longer waiting for output
			return
		case <-retry.C:
			// Time to retry at the given frequency
			result, err := m.ssmClient.GetCommandInvocation(&ssm.GetCommandInvocationInput{
				CommandId:  aws.String(commandID),
				InstanceId: aws.String(instanceID),
			})
			if out, ok := newCommandOutput(result, err); ok {
				c <- out
				return
			}
		}
	}
}

func newCommandOutput(result *ssm.GetCommandInvocationOutput, err error) (*CommandOutput, bool) {
	out := &CommandOutput{
		InstanceID: aws.StringValue(result.InstanceId),
		Status:     aws.StringValue(result.StatusDetails),
		Output:     "",
		Error:      err,
	}

	if err != nil {
		return out, true
	}

	switch out.Status {
	case "Pending", "InProgress", "Delayed":
		return out, false
	case "Cancelled":
		out.Output = "Command was cancelled"
		return out, true
	case "Success":
		out.Output = aws.StringValue(result.StandardOutputContent)
		return out, true
	case "Failed":
		out.Output = aws.StringValue(result.StandardErrorContent)
		return out, true
	default:
		out.Error = fmt.Errorf("Unrecoverable status: %s", out.Status)
		return out, true
	}
}

func (m *Manager) readS3Output(bucket, key string) (string, error) {
	output, err := m.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", err
	}

	defer output.Body.Close()
	b := bytes.NewBuffer(nil)

	if _, err := io.Copy(b, output.Body); err != nil {
		return "", err
	}
	return b.String(), nil
}
