package manager

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"io"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

type Manager struct {
	SSM           ssmiface.SSMAPI
	S3            s3manageriface.DownloaderAPI
	Region        string
	PollFrequency int
	PollTimeout   int
}

// The return type transmitted over a channel when fetching output.
type Output struct {
	InstanceId string
	Status     string
	Output     string
	Error      error
}

// Create a new manager.
func NewManager(sess *session.Session, region string, frequency int, timeout int) *Manager {
	return &Manager{
		SSM:           ssm.New(sess, &aws.Config{Region: aws.String(region)}),
		S3:            s3manager.NewDownloader(sess),
		Region:        region,
		PollFrequency: frequency,
		PollTimeout:   timeout,
	}
}

// Fetch a list of instances managed by SSM. Paginates until all responses have been collected.
func (self *Manager) GetInstances(limit int) ([]*ssm.InstanceInformation, error) {
	if min := 5; min > limit {
		limit = min
	}
	input := &ssm.DescribeInstanceInformationInput{
		MaxResults: aws.Int64(int64(limit)),
	}

	var out []*ssm.InstanceInformation
	for {
		response, err := self.SSM.DescribeInstanceInformation(input)
		if err != nil {
			return nil, err
		}
		for _, instance := range response.InstanceInformationList {
			out = append(out, instance)
		}
		if response.NextToken == nil {
			break
		}
		input.NextToken = response.NextToken
	}
	return out, nil
}

// Lists all instances and writes the tabulated output to
// the given interface.
func (self *Manager) List(out io.Writer, limit int) error {
	// Get all instances
	instances, err := self.GetInstances(limit)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(out, 0, 8, 1, ' ', 0)
	header := []string{
		"Instance ID",
		"Platform",
		"Version",
		"IP",
		"Last pinged",
	}

	if _, err := fmt.Fprintln(w, strings.Join(header, "\t|\t")); err != nil {
		return err
	}

	for _, instance := range instances {
		fields := []string{
			aws.StringValue(instance.InstanceId),
			aws.StringValue(instance.PlatformName),
			aws.StringValue(instance.PlatformVersion),
			aws.StringValue(instance.IPAddress),
			aws.TimeValue(instance.LastPingDateTime).Format("2006-01-02"),
		}
		if _, err := fmt.Fprintln(w, strings.Join(fields, "\t|\t")); err != nil {
			return err
		}
	}

	if err := w.Flush(); err != nil {
		return err
	}
	return nil
}

// Run command on the given instance ids.
func (self *Manager) Run(targets []string, command string) (string, error) {
	input := &ssm.SendCommandInput{
		InstanceIds:  aws.StringSlice(targets),
		DocumentName: aws.String("AWS-RunShellScript"),
		Comment:      aws.String("Interactive command."),
		Parameters:   map[string][]*string{"commands": []*string{aws.String(command)}},
	}

	res, err := self.SSM.SendCommand(input)
	if err != nil {
		return "", err
	}

	return aws.StringValue(res.Command.CommandId), nil
}

// Abort command on the given instance ids.
func (self *Manager) Abort(instanceIds []string, commandId string) error {
	_, err := self.SSM.CancelCommand(&ssm.CancelCommandInput{
		CommandId:   aws.String(commandId),
		InstanceIds: aws.StringSlice(instanceIds),
	})
	if err != nil {
		return err
	}
	return nil
}

// Output fetches standard output (or standard error) depending on the status
// for the given command. The results are sent over a channel, which is closed
// when all output has been fetched, or the timeout limit has been reached.
func (self *Manager) Output(instanceIds []string, commandId string, output chan Output) {
	defer close(output)
	var wg sync.WaitGroup
	for _, instanceId := range instanceIds {
		wg.Add(1)
		go self.getInstanceOutput(instanceId, commandId, output, &wg)
	}
	wg.Wait()
	return
}

// Fetch output from a command invocation on an instance.
func (self *Manager) getInstanceOutput(instanceId string, commandId string, output chan Output, wg *sync.WaitGroup) {
	defer wg.Done()
	retry := time.NewTicker(time.Millisecond * time.Duration(self.PollFrequency))
	limit := time.NewTimer(time.Second * time.Duration(self.PollTimeout))

	o := Output{
		InstanceId: instanceId,
		Status:     "",
		Output:     "",
		Error:      nil,
	}

	for {
		select {
		case <-limit.C:
			o.Error = errors.New("Timeout reached when awaiting output.")
			output <- o
			return
		case <-retry.C:
			result, err := self.SSM.GetCommandInvocation(&ssm.GetCommandInvocationInput{
				CommandId:  aws.String(commandId),
				InstanceId: aws.String(instanceId),
			})
			if err != nil {
				o.Error = err
				output <- o
				return
			}
			if out, done := processOutput(result); done {
				output <- out
				return
			}
		}
	}
}

// Internal function to process output from GetCommandInvocation.
func processOutput(input *ssm.GetCommandInvocationOutput) (Output, bool) {
	out := Output{
		InstanceId: aws.StringValue(input.InstanceId),
		Status:     aws.StringValue(input.StatusDetails),
		Output:     "",
		Error:      nil,
	}

	switch out.Status {
	case "Cancelled":
		out.Output = "Command was aborted"
		return out, true
	case "Success":
		out.Output = aws.StringValue(input.StandardOutputContent)
		return out, true
	case "Failed":
		out.Output = aws.StringValue(input.StandardErrorContent)
		return out, true
	case "Pending", "InProgress", "Delayed":
		return out, false
	default:
		out.Error = errors.New(fmt.Sprintf("Unrecoverable status: %s", out.Status))
		return out, true
	}
}
