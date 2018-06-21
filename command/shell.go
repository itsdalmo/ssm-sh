package command

import (
	"context"
	"fmt"
	"io"
	"os"
	"io/ioutil"
	"strings"
	"os/exec"
	"encoding/base64"
	"path/filepath"

	// "os/signal"
	// "sync"

	"github.com/chzyer/readline"
	"github.com/itsdalmo/ssm-sh/manager"
	"github.com/pkg/errors"
	// "github.com/fsnotify/fsnotify"
)

type ShellCommand struct {
	SSMOpts    SSMOptions `group:"SSM options"`
	TargetOpts TargetOptions
}

func (command *ShellCommand) Execute([]string) error {
	var shellDocument = "AWS-RunShellScript"

	sess, err := newSession()

	if err != nil {
		return errors.Wrap(err, "failed to create new aws session")
	}

	opts, err := command.SSMOpts.Parse()
	if err != nil {
		return err
	}
	m := manager.NewManager(sess, Command.AwsOpts.Region, *opts)
	targets, err := setTargets(command.TargetOpts)
	if err != nil {
		return errors.Wrap(err, "failed to set targets")
	}

	var filters []*manager.TagFilter
	filters = append(filters, &manager.TagFilter{
		Key:    "platform",
		Values: []string{"windows"},
	})

	windowsTargets, err := m.FilterInstances(targets, filters)
	if len(targets) != len(windowsTargets) {
		errors.New("cannot mix windows and linux targets")
	}

	if len(windowsTargets) > 0 {
		fmt.Printf("Windows Targets\n\n")
		shellDocument = "AWS-RunPowerShellScript"
	}

	// (Parent) Context for the main thread and output channel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Catch sigterms to gracefully shut down
	var interrupts int
	abort := interruptHandler()

	// Configure readline
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "\033[31m»\033[0m ",
		HistoryFile:     "/tmp/ssh-sh.tmp",
		InterruptPrompt: "^C",
		EOFPrompt:       "^D",
	})
	if err != nil {
		panic(err)
	}
	defer rl.Close()

	fmt.Printf("Type 'exit' to exit. Use ctrl-c to abort running commands.\n\n")

	var nextCommand, cmd string
	for {
		if nextCommand == ""{
			cmd, err = rl.Readline()

			if err == readline.ErrInterrupt {
				continue
			} else if err == io.EOF {
				return nil
			}
		} else {
			cmd = nextCommand
			nextCommand = ""
		}

		var commandID, processFile string
		process := "shell"
		cmd = strings.TrimSpace(cmd)
		if len(cmd) == 0 {
			continue
		} else if cmd == "exit" {
			return nil
		} else if strings.HasPrefix(cmd, "edit ") {
			editCmd := "Get-Content " + cmd[5:]
			processFile = cmd[5:]
			commandID, err = m.RunCommand(targets, shellDocument, map[string]string{"commands": editCmd })
			process = "edit"
		} else{
			commandID, err = m.RunCommand(targets, shellDocument, map[string]string{"commands": cmd})
		}

		if err != nil {
			return errors.Wrap(err, "failed to Run command")
		}
		out := make(chan *manager.CommandOutput)
		go m.GetCommandOutput(ctx, targets, commandID, out)

	Polling:
		for {
			select {
			case <-abort:
				interrupts++
				err := m.AbortCommand(targets, commandID)
				if err != nil {
					return errors.Wrap(err, "failed to abort command on sigterm")
				}
			case output, open := <-out:
				if output == nil && !open {
					break Polling
				}
				nextCommand, err = processOutput(processFile, output, process)
				if err != nil {
					return errors.Wrap(err, "failed to process")
				}
			}
		}
	}
}

func processOutput(processFile string, output *manager.CommandOutput, process string) (string, error) {
	switch process {
		case "shell" : {
			if err := PrintCommandOutput(os.Stdout, output); err != nil {
				return "", errors.Wrap(err, "processOutput")
			}
			return "", nil
		}
		case "edit": {
			nextCommand, err := editCommand(processFile, output)
			if err != nil {
				return "", errors.Wrap(err, "processOutput")
			}
			return nextCommand, nil
		}
	}
	return "", nil
}

func editCommand(processFile string, output *manager.CommandOutput) (string, error) {
	fileContent := output.Output
	if strings.HasSuffix(fileContent, "--output truncated--") {
		return "", errors.New("output truncated, file is too large to edit")
	}
	tempFile, err := createTempfile(processFile, []byte(fileContent))
	if err != nil {
		return "", errors.Wrap(err, "editCommand")
	}
	if err := editFile(tempFile); err != nil {
		return "", errors.Wrap(err, "editCommand")
	}
	nextCommand, err := saveCommand(processFile, tempFile)
	if err != nil {
		return "", errors.Wrap(err, "editCommand")
	}

	return nextCommand, nil
}

func saveCommand(remoteFile string, localFile string) (string, error) {
	body, err := ioutil.ReadFile(localFile)
	if err != nil {
		return "", errors.Wrap(err, "saveCommand")
	}
	// TODO: size limits!
	// TODO: make this determined by target type ie, unix : "/usr/bin/base64 -d %s > %s"
	saveCmd := fmt.Sprintf("[System.Text.Encoding]::ASCII.GetString([System.Convert]::FromBase64String('%s')) | Out-File -Encoding ASCII '%s'", base64.StdEncoding.EncodeToString(body), remoteFile)
	return saveCmd, nil
}

func createTempfile(fileName string, body []byte) (string, error) {
	dir, err := ioutil.TempDir(os.TempDir(), "ssm-sh")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file")
	}
	// Sorry for windows
	tempFileName := filepath.Join(dir, filepath.Base(strings.Replace(fileName, "\\", string(os.PathSeparator), -1)))
	fmt.Println("temp file created : ", tempFileName)

	err = ioutil.WriteFile(tempFileName, body, os.ModePerm)
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file")
	}
	return tempFileName, nil
}

func editFile(path string) error{
	command := getDefaultEditor() + " " + path

	cmd := exec.Command("sh", "-c", command)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to edit")
	}

	return nil
}

func getDefaultEditor() string {
	return os.Getenv("EDITOR")
}
