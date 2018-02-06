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
	"github.com/fatih/color"
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
func (self *Manager) Run(instanceIds []string, command string) (string, error) {
	input := &ssm.SendCommandInput{
		InstanceIds:  aws.StringSlice(instanceIds),
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

// Wrapper for GetOutput which writes the output to the desired io.Writer. Interrupts are handled
// by calling Abort and waiting for the final output of the command.
func (self *Manager) Output(w io.Writer, instanceIds []string, commandId string, done <-chan bool) {
	out := make(chan Output)
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
func (self *Manager) GetOutput(instanceIds []string, commandId string, out chan<- Output, done <-chan bool) {
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
func (self *Manager) pollInstanceOutput(instanceId string, commandId string, wg *sync.WaitGroup, out chan<- Output, done <-chan bool) {
	defer wg.Done()
	retry := time.NewTicker(time.Millisecond * time.Duration(self.PollFrequency))
	limit := time.NewTimer(time.Second * time.Duration(self.PollTimeout))

	o := Output{
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
			result, err := self.SSM.GetCommandInvocation(&ssm.GetCommandInvocationInput{
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
func processInstanceOutput(input *ssm.GetCommandInvocationOutput) (Output, bool) {
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
