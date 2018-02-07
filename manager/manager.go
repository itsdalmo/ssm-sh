package manager

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"
	"github.com/fatih/color"
	"github.com/pkg/errors"
	"io"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

type Manager struct {
	ssmClient ssmiface.SSMAPI
	s3Client  s3iface.S3API
	region    string

	pollFrequency int
	pollTimeout   int
}

// The return type transmitted over a channel when fetching output.
type CommandOutput struct {
	InstanceId string
	Status     string
	Output     string
	Error      error
}

// Create a new manager.
func NewManager(sess *session.Session, region string, frequency int, timeout int) *Manager {
	config := &aws.Config{Region: aws.String(region)}
	return &Manager{
		ssmClient:     ssm.New(sess, config),
		s3Client:      s3.New(sess, config),
		region:        region,
		pollFrequency: frequency,
		pollTimeout:   timeout,
	}
}

// Create a new manager for testing purposes.
func NewTestManager(ssm ssmiface.SSMAPI, s3 s3iface.S3API) *Manager {
	return &Manager{
		ssmClient:     ssm,
		s3Client:      s3,
		region:        "eu-west-1",
		pollFrequency: 500,
		pollTimeout:   30,
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
		response, err := self.ssmClient.DescribeInstanceInformation(input)
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
func (self *Manager) Run(instanceIds []string, command string) (string, error) {
	input := &ssm.SendCommandInput{
		InstanceIds:  aws.StringSlice(instanceIds),
		DocumentName: aws.String("AWS-RunShellScript"),
		Comment:      aws.String("Interactive command."),
		Parameters:   map[string][]*string{"commands": []*string{aws.String(command)}},
	}

	res, err := self.ssmClient.SendCommand(input)
	if err != nil {
		return "", err
	}

	return aws.StringValue(res.Command.CommandId), nil
}

// Abort command on the given instance ids.
func (self *Manager) Abort(instanceIds []string, commandId string) error {
	_, err := self.ssmClient.CancelCommand(&ssm.CancelCommandInput{
		CommandId:   aws.String(commandId),
		InstanceIds: aws.StringSlice(instanceIds),
	})
	if err != nil {
		return err
	}
	return nil
}

// Wrapper for GetOutput which writes the output to the desired io.Writer. Interrupts are handled
// by calling Abort and waiting for the final output of the command.
func (self *Manager) Output(w io.Writer, instanceIds []string, commandId string, done <-chan bool) {
	out := make(chan CommandOutput)
	go self.GetOutput(instanceIds, commandId, out, done)

	header := color.New(color.Bold)
	for o := range out {
		header.Fprintf(w, "%s - %s:\n", o.InstanceId, o.Status)
		if o.Error != nil {
			fmt.Fprintf(w, "%s\n", o.Error)
			continue
		}
		fmt.Fprintf(w, "%s\n", o.Output)
	}
	return
}

// GetOutput fetches standard output (or standard error) depending on the status
// for the given command. The results are sent over a channel, which is closed
// when all output has been fetched, or the timeout limit has been reached.
func (self *Manager) GetOutput(instanceIds []string, commandId string, out chan<- CommandOutput, done <-chan bool) {
	var wg sync.WaitGroup
	var interrupts int

	// Channel in case we need to stop go routines immediately
	abort := make(chan bool)
	defer close(abort)

	// Spawn a go routine per instance id
	for _, instanceId := range instanceIds {
		wg.Add(1)
		go self.pollInstanceOutput(instanceId, commandId, &wg, out, abort)
	}

	// Use a go routine to wait for tasks to complete
	finished := make(chan bool)

	go func() {
		defer close(finished)
		defer close(out)
		wg.Wait()
		finished <- true

	}()

	// Main thread should call abort() if a message is sent to signify
	// that the main thread is done waiting. A 2nd signal on done should
	// propagate to the go routines to exit immediately.
	for {
		select {
		case <-finished:
			return
		case <-done:
			if interrupts++; interrupts > 1 {
				for _ = range instanceIds {
					abort <- true
				}
				return
			}
			self.Abort(instanceIds, commandId)
			continue
		}
	}
}

// Fetch output from a command invocation on an instance.
func (self *Manager) pollInstanceOutput(instanceId string, commandId string, wg *sync.WaitGroup, out chan<- CommandOutput, done <-chan bool) {
	defer wg.Done()
	retry := time.NewTicker(time.Millisecond * time.Duration(self.pollFrequency))
	limit := time.NewTimer(time.Second * time.Duration(self.pollTimeout))

	o := CommandOutput{
		InstanceId: instanceId,
		Status:     "NA",
		Output:     "NA",
		Error:      nil,
	}

	for {
		select {
		case <-done:
			// Main thread is no longer waiting for output
			return
		case <-limit.C:
			// We have reached the timeout
			o.Error = errors.New("Timeout reached when awaiting output.")
			out <- o
			return
		case <-retry.C:
			// Time to retry at the given frequency
			result, err := self.ssmClient.GetCommandInvocation(&ssm.GetCommandInvocationInput{
				CommandId:  aws.String(commandId),
				InstanceId: aws.String(instanceId),
			})
			if err != nil {
				o.Error = err
				out <- o
				return
			}
			if o, ok := processInstanceOutput(result); ok {
				out <- o
				return
			}
		}
	}
}

// Internal function to process output from GetCommandInvocation on a single instance.
func processInstanceOutput(input *ssm.GetCommandInvocationOutput) (CommandOutput, bool) {
	out := CommandOutput{
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

func (self *Manager) readS3Output(bucket, key string) (string, error) {
	output, err := self.s3Client.GetObject(&s3.GetObjectInput{
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
