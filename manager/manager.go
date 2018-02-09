package manager

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/pkg/errors"
	"io"
	"sync"
	"time"
)

// TagFilter represents a key=value pair for AWS EC2 tags.
type TagFilter struct {
	Key    string
	Values []string
}

// Filter returns the ec2.Filter representation of the TagFilter.
func (t *TagFilter) Filter() *ec2.Filter {
	return &ec2.Filter{
		Name:   aws.String(fmt.Sprintf("tag:%s", t.Key)),
		Values: aws.StringSlice(t.Values),
	}
}

// CommandOutput is the return type transmitted over a channel when fetching output.
type CommandOutput struct {
	InstanceID string
	Status     string
	Output     string
	Error      error
}

// Manager handles the clients interfacing with AWS.
type Manager struct {
	ssmClient ssmiface.SSMAPI
	s3Client  s3iface.S3API
	ec2Client ec2iface.EC2API
	region    string
}

// NewManager creates a new Manager from an AWS session and region.
func NewManager(sess *session.Session, region string) *Manager {
	config := &aws.Config{Region: aws.String(region)}
	return &Manager{
		ssmClient: ssm.New(sess, config),
		s3Client:  s3.New(sess, config),
		ec2Client: ec2.New(sess, config),
		region:    region,
	}
}

// NewTestManager creates a new manager for testing purposes.
func NewTestManager(ssm ssmiface.SSMAPI, s3 s3iface.S3API, ec2 ec2iface.EC2API) *Manager {
	return &Manager{
		ssmClient: ssm,
		s3Client:  s3,
		ec2Client: ec2,
		region:    "eu-west-1",
	}
}

// ListInstances fetches a list of instances managed by SSM. Paginates until all responses have been collected.
func (m *Manager) ListInstances(limit int64, tagFilters []*TagFilter) ([]*Instance, error) {
	var out []*Instance

	input := &ssm.DescribeInstanceInformationInput{
		MaxResults: &limit,
	}

	for {
		response, err := m.ssmClient.DescribeInstanceInformation(input)
		if err != nil {
			return nil, errors.Wrap(err, "failed to describe instance information")
		}
		ssmInstances, ec2Instances, err := m.describeInstances(response.InstanceInformationList, tagFilters)
		if err != nil {
			return nil, errors.Wrap(err, "failed to retrieve ec2 instance information")
		}

		// NOTE: ec2Info will be a shorter list when filtering is applied.
		for k := range ec2Instances {
			out = append(out, NewInstance(ssmInstances[k], ec2Instances[k]))
		}
		if response.NextToken == nil {
			break
		}
		input.NextToken = response.NextToken
	}

	return out, nil
}

// describeInstances retrieves additional information about SSM managed instances from EC2.
func (m *Manager) describeInstances(instances []*ssm.InstanceInformation, tagFilters []*TagFilter) (map[string]*ssm.InstanceInformation, map[string]*ec2.Instance, error) {
	var ids []*string
	var filters []*ec2.Filter

	org := make(map[string]*ssm.InstanceInformation)
	out := make(map[string]*ec2.Instance)

	for _, instance := range instances {
		org[aws.StringValue(instance.InstanceId)] = instance
		ids = append(ids, instance.InstanceId)
	}

	filters = append(filters, &ec2.Filter{
		Name:   aws.String("instance-id"),
		Values: ids,
	})

	for _, f := range tagFilters {
		filters = append(filters, f.Filter())
	}

	input := &ec2.DescribeInstancesInput{
		Filters: filters,
	}

	for {
		response, err := m.ec2Client.DescribeInstances(input)
		if err != nil {
			return nil, nil, err
		}
		for _, reservation := range response.Reservations {
			for _, instance := range reservation.Instances {
				id := aws.StringValue(instance.InstanceId)
				out[id] = instance
			}
		}
		if response.NextToken == nil {
			break
		}
		input.NextToken = response.NextToken
	}

	return org, out, nil
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
