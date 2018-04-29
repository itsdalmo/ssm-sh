package manager

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"io/ioutil"
	"strings"
	"sync"
)

type MockEC2 struct {
	ec2iface.EC2API
	Instances map[string]*ec2.Instance
	Error     bool
}

func (mock *MockEC2) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	if mock.Error {
		return nil, errors.New("expected")
	}

	var out []*ec2.Instance
	var tmp []*ec2.Instance
	var ids []string
	var nameFilters []string

	for _, filter := range input.Filters {
		key := aws.StringValue(filter.Name)
		if key == "instance-id" {
			ids = aws.StringValueSlice(filter.Values)
		} else if key == "tag:Name" {
			nameFilters = aws.StringValueSlice(filter.Values)
		}
	}

	// Filter instance ids if a list was provided. If not, we
	// provide the entire list of instances.
	if ids != nil {
		for _, id := range ids {
			instance, ok := mock.Instances[id]
			if !ok {
				return nil, errors.New("instance id does not exist")
			}
			tmp = append(tmp, instance)
		}

	} else {
		for _, instance := range mock.Instances {
			tmp = append(tmp, instance)
		}
	}

	// If a tag filter was supplied (only Name is supported for testing),
	// filter instances which don't match.
	if nameFilters != nil && len(nameFilters) > 0 {
		for _, instance := range tmp {
			for _, tag := range instance.Tags {
				// Look for Name tag.
				if aws.StringValue(tag.Key) != "Name" {
					continue
				}
				// Once it is found, check whether it contains
				// any of the name filters (simple contains).
				name := aws.StringValue(tag.Value)
				for _, filter := range nameFilters {
					if strings.Contains(name, filter) {
						out = append(out, instance)
					}
				}
			}
		}

	} else {
		out = tmp
	}

	// NOTE: It should not matter if we have multiple reservations
	// and/or multiple instances per reservation.
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			{
				Instances: out,
			},
		},
		NextToken: nil,
	}, nil
}

type MockSSM struct {
	ssmiface.SSMAPI
	Instances           []*ssm.InstanceInformation
	Documents           []*ssm.DocumentIdentifier
	DocumentDescription *ssm.DocumentDescription
	NextToken           string
	CommandStatus       string
	CommandHistory      map[string]*struct {
		Command *ssm.Command
		Status  string
	}
	Error bool
	async sync.Mutex
}

func (mock *MockSSM) DescribeInstanceInformation(input *ssm.DescribeInstanceInformationInput) (*ssm.DescribeInstanceInformationOutput, error) {
	if mock.Error {
		return nil, errors.New("expected")
	}

	output := mock.Instances
	if input.MaxResults != nil {
		if i := int(*input.MaxResults); i < len(mock.Instances) {
			output = mock.Instances[:i]
		}
	}

	if mock.NextToken != "" {
		switch {
		case input.NextToken == nil:
			// Give an empty list on first response
			return &ssm.DescribeInstanceInformationOutput{
				InstanceInformationList: []*ssm.InstanceInformation{},
				NextToken:               aws.String(mock.NextToken),
			}, nil
		case *input.NextToken == mock.NextToken:
			return &ssm.DescribeInstanceInformationOutput{
				InstanceInformationList: output,
				NextToken:               nil,
			}, nil
		default:
			return nil, errors.New("Wrong token")
		}

	}
	return &ssm.DescribeInstanceInformationOutput{
		InstanceInformationList: output,
		NextToken:               nil,
	}, nil
}

func (mock *MockSSM) ListDocuments(input *ssm.ListDocumentsInput) (*ssm.ListDocumentsOutput, error) {
	if mock.Error {
		return nil, errors.New("expected")
	}

	var output []*ssm.DocumentIdentifier

	// filter output based on DocumentFilterList
	// only "Owner" is supported for testing
	if len(input.DocumentFilterList) > 0 {
		for _, filter := range input.DocumentFilterList {
			if aws.StringValue(filter.Key) == "Owner" {
				for _, document := range mock.Documents {
					if aws.StringValue(document.Owner) == aws.StringValue(filter.Value) {
						output = append(output, document)
					}
				}
			}
		}
	} else {
		output = mock.Documents
	}

	if input.MaxResults != nil {
		if i := int(*input.MaxResults); i < len(mock.Documents) {
			output = mock.Documents[:i]
		}
	}

	if mock.NextToken != "" {
		switch {
		case input.NextToken == nil:
			// Give an empty list on first response
			return &ssm.ListDocumentsOutput{
				DocumentIdentifiers: []*ssm.DocumentIdentifier{},
				NextToken:           aws.String(mock.NextToken),
			}, nil
		case *input.NextToken == mock.NextToken:
			return &ssm.ListDocumentsOutput{
				DocumentIdentifiers: output,
				NextToken:           nil,
			}, nil
		default:
			return nil, errors.New("Wrong token")
		}

	}
	return &ssm.ListDocumentsOutput{
		DocumentIdentifiers: output,
		NextToken:           nil,
	}, nil
}

func (mock *MockSSM) DescribeDocument(input *ssm.DescribeDocumentInput) (*ssm.DescribeDocumentOutput, error) {
	if mock.Error {
		return nil, errors.New("expected")
	}

	if aws.StringValue(input.Name) != aws.StringValue(mock.DocumentDescription.Name) {
		return nil, errors.New("expected")
	}

	return &ssm.DescribeDocumentOutput{
		Document: mock.DocumentDescription,
	}, nil
}

func (mock *MockSSM) SendCommand(input *ssm.SendCommandInput) (*ssm.SendCommandOutput, error) {
	if mock.Error {
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

	mock.async.Lock()
	defer mock.async.Unlock()

	id := fmt.Sprintf("command-%d", len(mock.CommandHistory)+1)
	command := &ssm.Command{
		CommandId:          aws.String(id),
		Comment:            input.Comment,
		DocumentName:       input.DocumentName,
		InstanceIds:        input.InstanceIds,
		OutputS3BucketName: input.OutputS3BucketName,
		OutputS3KeyPrefix:  input.OutputS3KeyPrefix,
	}
	mock.CommandHistory[id] = &struct {
		Command *ssm.Command
		Status  string
	}{
		Command: command,
		Status:  mock.CommandStatus,
	}
	return &ssm.SendCommandOutput{Command: command}, nil
}

func (mock *MockSSM) CancelCommand(input *ssm.CancelCommandInput) (*ssm.CancelCommandOutput, error) {
	if mock.Error {
		return nil, errors.New("expected")
	}

	if input.CommandId == nil {
		return nil, errors.New("Missing CommandId")
	}

	if input.InstanceIds == nil || len(input.InstanceIds) == 0 {
		return nil, errors.New("Missing InstanceIds")
	}

	mock.async.Lock()
	defer mock.async.Unlock()

	id := aws.StringValue(input.CommandId)
	cmd, ok := mock.CommandHistory[id]
	if !ok {
		return nil, errors.New("invalid commandId")
	}
	cmd.Status = "Cancelled"

	return &ssm.CancelCommandOutput{}, nil
}

func (mock *MockSSM) GetCommandInvocation(input *ssm.GetCommandInvocationInput) (*ssm.GetCommandInvocationOutput, error) {
	if mock.Error {
		return nil, errors.New("expected")
	}

	if input.CommandId == nil {
		return nil, errors.New("Missing CommandId")
	}

	if input.InstanceId == nil {
		return nil, errors.New("Missing InstanceId")
	}

	mock.async.Lock()
	defer mock.async.Unlock()

	id := aws.StringValue(input.CommandId)
	cmd, ok := mock.CommandHistory[id]
	if !ok {
		return nil, errors.New("invalid commandId")
	}

	return &ssm.GetCommandInvocationOutput{
		InstanceId:            input.InstanceId,
		StatusDetails:         aws.String(cmd.Status),
		StandardOutputContent: aws.String("example standard output"),
		StandardErrorContent:  aws.String("example standard error"),
	}, nil
}

type MockS3 struct {
	s3iface.S3API
	Error bool
}

func (mock *MockS3) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	if mock.Error {
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
