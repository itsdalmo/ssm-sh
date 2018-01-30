package main

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/jessevdk/go-flags"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"
)

var (
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

func Logging(infoHandle, warningHandle, errorHandle io.Writer) {
	Info = log.New(infoHandle, "[INFO] ", log.Ldate|log.Ltime)
	Warning = log.New(warningHandle, "[WARNING] ", log.Ldate|log.Ltime)
	Error = log.New(errorHandle, "[ERROR] ", log.Ldate|log.Ltime)
}

type Instance struct {
	InstanceId      string    `json:"instanceId"`
	PlatformName    string    `json:"platformName"`
	PlatformVersion string    `json:"platformVersion"`
	IpAddress       string    `json:"ipAddress"`
	PingTime        time.Time `json:"pingTime"`
}

func NewInstance(input *ssm.InstanceInformation) *Instance {
	return &Instance{
		InstanceId:      *input.InstanceId,
		PlatformName:    *input.PlatformName,
		PlatformVersion: *input.PlatformVersion,
		IpAddress:       *input.IPAddress,
		PingTime:        *input.LastPingDateTime,
	}
}

func (self *Instance) String() string {
	return fmt.Sprintf(
		"%-20s | %-20s | %-7s | %-14s | %-10s",
		self.InstanceId,
		self.PlatformName,
		self.PlatformVersion,
		self.IpAddress,
		self.PingTime.Format("2006-01-02"),
	)
}

func ListTargetsHeader() string {
	return fmt.Sprintf(
		"%-20s | %-20s | %-7s | %-14s | %-10s",
		"Instance ID",
		"Platform",
		"Version",
		"IP",
		"Last pinged",
	)
}

func RequestTarget(reader *bufio.Reader, svc *ssm.SSM) (string, error) {
	response, err := svc.DescribeInstanceInformation(&ssm.DescribeInstanceInformationInput{})
	if err != nil {
		return "", err
	}
	fmt.Println(ListTargetsHeader())
	for _, instance := range response.InstanceInformationList {
		fmt.Println(NewInstance(instance))
	}

	fmt.Print("Chose target (instance id): ")
	target, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(target), nil
}

type Target struct {
	ssm       *ssm.SSM
	target    string
	frequency int
	timeout   int
}

func (self *Target) Run(command string) (string, error) {
	cmd := &ssm.SendCommandInput{
		Comment:      aws.String("Interactive command."),
		DocumentName: aws.String("AWS-RunShellScript"),
		InstanceIds:  []*string{aws.String(self.target)},
	}
	cmd.Parameters = map[string][]*string{"commands": []*string{aws.String(command)}}
	out, err := self.ssm.SendCommand(cmd)
	if err != nil {
		return "", err
	}
	return aws.StringValue(out.Command.CommandId), nil
}

func (self *Target) Abort(id string) error {
	_, err := self.ssm.CancelCommand(&ssm.CancelCommandInput{
		CommandId:   aws.String(id),
		InstanceIds: []*string{aws.String(self.target)},
	})
	if err != nil {
		return err
	}
	return nil
}

func (self *Target) PollOutput(id string, sigterm chan os.Signal) (string, error) {
	ticker := time.NewTicker(time.Millisecond * time.Duration(self.frequency))
	limit := time.NewTimer(time.Second * time.Duration(self.timeout))

	for {
		select {
		case <-sigterm:
			err := self.Abort(id)
			if err != nil {
				return "", err
			}
			continue
		case <-limit.C:
			return "", errors.New("Timeout reached when awaiting output.")
		case <-ticker.C:
			out, err := self.ssm.GetCommandInvocation(&ssm.GetCommandInvocationInput{
				CommandId:  aws.String(id),
				InstanceId: aws.String(self.target),
			})
			if err != nil {
				return "", err
			}

			switch status := aws.StringValue(out.StatusDetails); status {
			case "Success":
				return aws.StringValue(out.StandardOutputContent), nil
			case "Failed":
				return aws.StringValue(out.StandardErrorContent), nil
			case "Cancelled":
				return "Command was aborted.", nil
			case "Pending", "InProgress", "Delayed":
				break
			default:
				return "", errors.New(fmt.Sprintf("Unrecoverable status: %s", status))
			}
		}
	}
}

func PromptUser(input chan string, request chan bool, reader *bufio.Reader) {
	for _ = range request {
		fmt.Print("$ ")
		command, err := reader.ReadString('\n')
		if err != nil {
			// TODO: Better error handling
			Error.Println(err)
			os.Exit(1)
		}
		input <- command
	}
}

type Options struct {
	Profile   string `short:"p" long:"profile" description:"AWS Profile to use. (If you are not using Vaulted)."`
	Region    string `short:"r" long:"region" description:"Region to target." default:"eu-west-1"`
	Frequency int    `short:"f" long:"frequency" description:"Polling frequency (millseconds to wait between requests)." default:"500"`
	Timeout   int    `short:"t" long:"timeout" description:"Seconds to wait for command result before timing out." default:"30"`
}

func main() {
	var opts Options
	if _, err := flags.Parse(&opts); err != nil {
		os.Exit(1)
	}
	Logging(os.Stdout, os.Stdout, os.Stderr)

	// Load AWS credentials
	conf := session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}
	if opts.Profile != "" {
		Info.Printf("Using AWS profile: %s\n", opts.Profile)
		conf.Profile = opts.Profile
	}
	sess, err := session.NewSessionWithOptions(conf)
	if err != nil {
		Error.Printf("Failed to create AWS Session: %s\n", err)
		os.Exit(1)
	}
	Info.Printf("AWS credentials loaded\n")
	svc := ssm.New(sess, &aws.Config{Region: aws.String(opts.Region)})

	// Request that user sets target
	reader := bufio.NewReader(os.Stdin)
	instanceId, err := RequestTarget(reader, svc)
	if err != nil {
		Error.Printf("Failed to set target: %s\n", err)
		os.Exit(1)
	}

	Info.Printf("Targeting: '%s'\n", instanceId)
	target := &Target{
		ssm:       svc,
		target:    instanceId,
		frequency: opts.Frequency,
		timeout:   opts.Timeout,
	}

	// Start user prompt - use channel to trigger prompt and return input.
	input := make(chan string)
	request := make(chan bool)
	go PromptUser(input, request, reader)

	// Catch ctrl-c
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, os.Interrupt)

	for {
		// Start the prompt
		request <- true

		select {
		case <-sigterm:
			Info.Println("Ctrl-c received. Quitting.")
			return
		case cmd := <-input:
			// Just continue if given empty input
			if strings.TrimSpace(cmd) == "" {
				continue
			}
			id, err := target.Run(cmd)
			if err != nil {
				Error.Printf("Failed to send command: %s\n", err)
				os.Exit(1)
			}
			out, err := target.PollOutput(id, sigterm)
			if err != nil {
				Warning.Printf("Failed to poll output of %s:\n %s\n", id, err)
			}
			fmt.Println(out)
		}
	}
}
