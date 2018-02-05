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

type Output struct {
	InstanceId string
	Status     string
	Output     string
	Error      error
}

func NewManager(sess *session.Session, region string, frequency int, timeout int) *Manager {
	return &Manager{
		SSM:           ssm.New(sess, &aws.Config{Region: aws.String(region)}),
		S3:            s3manager.NewDownloader(sess),
		Region:        region,
		PollFrequency: frequency,
		PollTimeout:   timeout,
	}
}

func (self *Manager) GetInstances(limit int) ([]*ssm.InstanceInformation, error) {
	if min := 5; min > limit {
		limit = min
	}
	input := &ssm.DescribeInstanceInformationInput{
		MaxResults: aws.Int64(int64(limit)),
	}

	// Paginate and add instances to a flat slice.
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

func (self *Manager) Abort(targets []string, id string) error {
	_, err := self.SSM.CancelCommand(&ssm.CancelCommandInput{
		CommandId:   aws.String(id),
		InstanceIds: aws.StringSlice(targets),
	})
	if err != nil {
		return err
	}
	return nil
}

func (self *Manager) Output(targets []string, id string, output chan Output) {
	defer close(output)
	var wg sync.WaitGroup
	for _, target := range targets {
		wg.Add(1)
		go self.getInstanceOutput(target, id, output, &wg)
	}
	wg.Wait()
	return
}

func (self *Manager) getInstanceOutput(target string, id string, output chan Output, wg *sync.WaitGroup) {
	defer wg.Done()
	retry := time.NewTicker(time.Millisecond * time.Duration(self.PollFrequency))
	limit := time.NewTimer(time.Second * time.Duration(self.PollTimeout))

	for {
		select {
		case <-limit.C:
			output <- Output{target, "", "", errors.New("Timeout reached when awaiting output.")}
			return
		case <-retry.C:
			out, err := self.SSM.GetCommandInvocation(&ssm.GetCommandInvocationInput{
				CommandId:  aws.String(id),
				InstanceId: aws.String(target),
			})
			if err != nil {
				output <- Output{target, "", "", err}
				return
			}

			switch status := aws.StringValue(out.StatusDetails); status {
			case "Success":
				output <- Output{target, status, aws.StringValue(out.StandardOutputContent), nil}
				return
			case "Failed":
				output <- Output{target, status, aws.StringValue(out.StandardErrorContent), nil}
				return
			case "Cancelled":
				output <- Output{target, status, "Command was aborted.", nil}
				return
			case "Pending", "InProgress", "Delayed":
				break
			default:
				output <- Output{target, status, "", errors.New(fmt.Sprintf("Unrecoverable status: %s", status))}
				return
			}
		}
	}
}
