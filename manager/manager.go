package manager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/pkg/errors"
)

// TagFilter represents a key=value pair for AWS EC2 tags.
type TagFilter struct {
	Key    string
	Values []string
}

// Filter returns the ec2.Filter representation of the TagFilter.
func (t *TagFilter) Filter() *ec2.Filter {
	return &ec2.Filter{
		Name:   aws.String(t.Key),
		Values: aws.StringSlice(t.Values),
	}
}

// CommandOutput is the return type transmitted over a channel when fetching output.
type CommandOutput struct {
	InstanceID string
	Status     string
	Output     string
	OutputUrl  string
	StdErr     string
	StdErrUrl  string
	Error      error
}

// Manager handles the clients interfacing with AWS.
type Manager struct {
	ssmClient    ssmiface.SSMAPI
	s3Client     s3iface.S3API
	ec2Client    ec2iface.EC2API
	extendOutput bool
	region       string
	s3Bucket     string
	s3KeyPrefix  string
}

type Opts struct {
	ExtendOutput bool
	S3Bucket     string
	S3KeyPrefix  string
}

// NewManager creates a new Manager from an AWS session and region.
func NewManager(sess *session.Session, region string, opts Opts) *Manager {
	awsCfg := &aws.Config{Region: aws.String(region)}
	m := &Manager{
		ssmClient: ssm.New(sess, awsCfg),
		s3Client:  s3.New(sess, awsCfg),
		ec2Client: ec2.New(sess, awsCfg),
		region:    region,
	}
	m.extendOutput = opts.ExtendOutput
	m.s3Bucket = opts.S3Bucket
	m.s3KeyPrefix = opts.S3KeyPrefix
	return m
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

// TODO: rewrite describeInstances so that we could use it inside FilterInstances, and then in turn use FilterInstances inside ListInstances. Makes things a little more DRY
// FilterInstances filters a given list of instances.
func (m *Manager) FilterInstances(instanceIds []string, tagFilters []*TagFilter) ([]string, error) {
	var in []*string
	var out []string
	var filters []*ec2.Filter

	for _, f := range tagFilters {
		filters = append(filters, f.Filter())
	}
	for _, i := range instanceIds {
		in = append(in, &i)
	}
	input := &ec2.DescribeInstancesInput{
		Filters:     filters,
		InstanceIds: in,
	}

	for {
		response, err := m.ec2Client.DescribeInstances(input)
		if err != nil {
			return nil, err
		}
		for _, reservation := range response.Reservations {
			for _, instance := range reservation.Instances {
				out = append(out, *instance.InstanceId)
			}
		}
		if response.NextToken == nil {
			break
		}
		input.NextToken = response.NextToken
	}

	return out, nil
}

// ListDocuments fetches a list of documents managed by SSM. Paginates until all responses have been collected.
func (m *Manager) ListDocuments(limit int64, documentFilters []*ssm.DocumentFilter) ([]*DocumentIdentifier, error) {
	var out []*DocumentIdentifier

	input := &ssm.ListDocumentsInput{
		MaxResults:         &limit,
		DocumentFilterList: documentFilters,
	}

	for {
		response, err := m.ssmClient.ListDocuments(input)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list document")
		}

		for _, document := range response.DocumentIdentifiers {
			out = append(out, NewDocumentIdentifier(document))
		}
		if response.NextToken == nil {
			break
		}
		input.NextToken = response.NextToken
	}

	return out, nil
}

// DescribeDocument lists information for a specific document managed by SSM.
func (m *Manager) DescribeDocument(name string) (*DocumentDescription, error) {

	input := &ssm.DescribeDocumentInput{
		Name: aws.String(name),
	}

	response, err := m.ssmClient.DescribeDocument(input)
	if err != nil {
		return nil, errors.Wrap(err, "failed to describe document")
	}

	document := NewDocumentDescription(response.Document)

	return document, nil
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

	for _, f := range tagFilters {
		filters = append(filters, f.Filter())
	}

	input := &ec2.DescribeInstancesInput{
		Filters:     filters,
		InstanceIds: ids,
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
func (m *Manager) RunCommand(instanceIds []string, name string, parameters map[string]string) (string, error) {

	var params map[string][]*string

	if len(parameters) > 0 {
		params = make(map[string][]*string)
		for k, v := range parameters {
			params[k] = aws.StringSlice([]string{v})
		}
	}

	input := &ssm.SendCommandInput{
		InstanceIds:  aws.StringSlice(instanceIds),
		DocumentName: aws.String(name),
		Comment:      aws.String("Document triggered through ssm-sh."),
		Parameters:   params,
	}
	if m.s3Bucket != "" {
		input.OutputS3BucketName = aws.String(m.s3Bucket)
	}
	if m.s3KeyPrefix != "" {
		input.OutputS3KeyPrefix = aws.String(m.s3KeyPrefix)
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
			if out, ok := m.newCommandOutput(result, err); ok {
				c <- out
				return
			}
		}
	}
}

func (m *Manager) newCommandOutput(result *ssm.GetCommandInvocationOutput, err error) (*CommandOutput, bool) {
	out := &CommandOutput{
		InstanceID: aws.StringValue(result.InstanceId),
		Status:     aws.StringValue(result.StatusDetails),
		Output:     "",
		StdErr:     "",
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
	case "Success", "Failed":
		out.Output = aws.StringValue(result.StandardOutputContent)
		out.OutputUrl = aws.StringValue(result.StandardOutputUrl)
		out.StdErr = aws.StringValue(result.StandardErrorContent)
		out.StdErrUrl = aws.StringValue(result.StandardErrorUrl)
		if m.extendOutput {
			return m.extendTruncatedOutput(*out), true
		}
		return out, true
	default:
		out.Error = fmt.Errorf("Unrecoverable status: %s", out.Status)
		return out, true
	}
}

func (m *Manager) extendTruncatedOutput(out CommandOutput) *CommandOutput {
	const truncationMarker = "--output truncated--"
	if strings.Contains(out.Output, truncationMarker) {
		s3out, err := m.readOutput(out.OutputUrl)
		if err != nil {
			out.Error = errors.Wrap(err, "failed to fetch extended output")
		}
		out.Output = s3out
		return &out
	}
	return &out
}

func (m *Manager) readOutput(url string) (string, error) {
	regex := regexp.MustCompile(`://s3[\-a-z0-9]*\.amazonaws.com/([^/]+)/(.+)|://([^.]+)\.s3\.amazonaws\.com/(.+)`)
	matches := regex.FindStringSubmatch(url)
	if len(matches) == 0 {
		return "", errors.Errorf("failed due to unexpected s3 url pattern: %s", url)
	}
	bucket := matches[1]
	key := matches[2]
	out, err := m.readS3Object(bucket, key)
	if err != nil {
		return "", errors.Wrapf(err, "failed to fetch s3 object: %s", url)
	}
	return out, nil
}

func (m *Manager) readS3Object(bucket, key string) (string, error) {
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
