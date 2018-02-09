package command

import (
	"bufio"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/fatih/color"
	"github.com/itsdalmo/ssm-sh/manager"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"
)

// Create a new AWS session
func newSession() (*session.Session, error) {
	opts := session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}
	if Command.AwsOpts.Profile != "" {
		opts.Profile = Command.AwsOpts.Profile
	}
	sess, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// Combine target flags
func targetFlagHelper(opts TargetOptions) ([]string, error) {
	var targets []string
	targets = opts.Targets

	if opts.TargetFile != "" {
		content, err := ioutil.ReadFile(opts.TargetFile)
		if err != nil {
			return nil, err
		}
		lines := strings.TrimSpace(string(content))
		for _, line := range strings.Split(lines, "\n") {
			targets = append(targets, line)
		}
	}
	return targets, nil
}

// PrintCommandOutput writes the output from command invocations.
func PrintCommandOutput(wrt io.Writer, output *manager.CommandOutput) error {
	header := color.New(color.Bold)
	if _, err := header.Fprintf(wrt, "\n%s - %s:\n", output.InstanceID, output.Status); err != nil {
		return err
	}
	if output.Error != nil {
		if _, err := fmt.Fprintf(wrt, "%s\n", output.Error); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(wrt, "%s\n", output.Output); err != nil {
		return err
	}
	return nil
}

// PrintInstances writes the output from ListInstances.
func PrintInstances(wrt io.Writer, instances []*manager.Instance) error {
	w := tabwriter.NewWriter(wrt, 0, 8, 1, ' ', 0)
	header := []string{
		"Instance ID",
		"Platform",
		"Version",
		"IP",
		"Status",
		"Last pinged",
	}

	if _, err := fmt.Fprintln(w, strings.Join(header, "\t|\t")); err != nil {
		return err
	}
	for _, instance := range instances {
		if _, err := fmt.Fprintln(w, instance.TabString()); err != nil {
			return err
		}
	}
	err := w.Flush()
	return err

}

func interruptHandler() <-chan bool {
	abort := make(chan bool)
	sigterm := make(chan os.Signal)
	signal.Notify(sigterm, os.Interrupt)

	go func() {
		defer signal.Stop(sigterm)
		defer close(sigterm)
		defer close(abort)

		// Use a threshold for time since last signal
		// to avoid multiple SIGTERM when pressing ctrl+c
		// on a keyboard.
		var last time.Time
		threshold := 50 * time.Millisecond

		for range sigterm {
			if time.Since(last) < threshold {
				continue
			}
			abort <- true
			last = time.Now()
		}
	}()
	return abort
}

func userPrompt(r *bufio.Reader) string {
	for {
		fmt.Print("$ ")
		command, err := r.ReadString('\n')
		if err != nil {
			continue
		}
		cmd := strings.TrimSpace(command)
		if cmd == "" {
			continue
		}
		return cmd
	}
}
