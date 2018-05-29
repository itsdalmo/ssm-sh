package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/fatih/color"
	"github.com/itsdalmo/ssm-sh/manager"
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

// Set targets
func setTargets(options TargetOptions) ([]string, error) {
	var instances []manager.Instance
	var targets []string
	if options.TargetFile != "" {
		content, err := ioutil.ReadFile(options.TargetFile)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(content, &instances); err != nil {
			return nil, err
		}
		for _, instance := range instances {
			targets = append(targets, instance.ID())
		}
	}

	for _, target := range options.Targets {
		targets = append(targets, target)
	}

	if len(targets) == 0 {
		return nil, errors.New("no targets set")
	}

	fmt.Printf("Initialized with targets: %s\n", targets)

	return targets, nil

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
	if _, err := fmt.Fprintf(wrt, "%s\n", output.StdErr); err != nil {
		return err
	}
	if output.OutputUrl != "" {
		if _, err := fmt.Fprintf(wrt, "(Output URL: %s)\n", output.OutputUrl); err != nil {
			return err
		}
	}
	return nil
}

// PrintInstances writes the output from ListInstances.
func PrintInstances(wrt io.Writer, instances []*manager.Instance) error {
	w := tabwriter.NewWriter(wrt, 0, 8, 1, ' ', 0)
	header := []string{
		"Instance ID",
		"Name",
		"State",
		"Image ID",
		"Platform",
		"Platform Description",
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

// PrintDocuments writes the output from ListDocuments.
func PrintDocuments(wrt io.Writer, documents []*manager.DocumentIdentifier) error {
	w := tabwriter.NewWriter(wrt, 0, 8, 1, ' ', 0)
	header := []string{
		"Name",
		"Owner",
		"Document Version",
		"Document Format",
		"Document Type",
		"Schema Version",
		"Target Type",
	}

	if _, err := fmt.Fprintln(w, strings.Join(header, "\t|\t")); err != nil {
		return err
	}
	for _, document := range documents {
		if _, err := fmt.Fprintln(w, document.TabString()); err != nil {
			return err
		}
	}
	err := w.Flush()
	return err

}

// PrintDocumentDescription writes the output from DescribeDocument.
func PrintDocumentDescription(wrt io.Writer, document *manager.DocumentDescription) error {
	w := tabwriter.NewWriter(wrt, 0, 8, 1, ' ', 0)

	header := []string{
		"Name",
		"Description",
		"Owner",
		"Document Version",
		"Document Format",
		"Document Type",
		"Schema Version",
		"Target Type",
	}

	if _, err := fmt.Fprintln(w, strings.Join(header, "\t|\t")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, document.TabString()); err != nil {
		return err
	}

	if len(document.Parameters) > 0 {
		if err := w.Flush(); err != nil {
			return err
		}

		fmt.Fprintf(wrt, "\nParameters:\n")
		parameterHeader := []string{
			"Name",
			"Type",
			"DefaultValue",
			"Description",
		}
		if _, err := fmt.Fprintln(w, strings.Join(parameterHeader, "\t|\t")); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, document.ParametersTabString()); err != nil {
			return err
		}
	}

	err := w.Flush()
	return err
}

// WriteInstances writes the output of ListInstances to a file as JSON.
func WriteInstances(wrt io.Writer, instances []*manager.Instance) error {
	w := json.NewEncoder(wrt)
	err := w.Encode(instances)
	return err
}
