package command

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"time"
)

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

		for _ = range sigterm {
			if time.Since(last) < threshold {
				continue
			}
			abort <- true
			last = time.Now()
		}
	}()
	return abort
}

func targetFlagHelper(opts SsmOptions) ([]string, error) {
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

func userPrompt(input chan string, request chan bool) {
	defer close(input)

	reader := bufio.NewReader(os.Stdin)

	for _ = range request {
		for {
			fmt.Print("$ ")
			command, err := reader.ReadString('\n')
			if err != nil {
				continue
			}
			cmd := strings.TrimSpace(command)
			if cmd == "" {
				continue
			}
			input <- cmd
			break
		}
	}
}
