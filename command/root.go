package command

var Command RootCommand

type RootCommand struct {
	Version          func()                  `short:"v" long:"version" description:"Print the version and exit."`
	List             ListCommand             `command:"list" alias:"ls" description:"List managed instances."`
	Shell            ShellCommand            `command:"shell" alias:"sh" description:"Start an interactive shell."`
	Run              RunCommand              `command:"run" description:"Run a command on the targeted instances."`
	ListDocuments    ListDocumentsCommand    `command:"list-documents" alias:"ld" description:"List available documents in ssm."`
	RunDocument      RunDocumentCommand      `command:"run-document" description:"Runs a document from ssm."`
	DescribeDocument DescribeDocumentCommand `command:"describe-document" description:"Description a document from ssm."`
	AwsOpts          AwsOptions              `group:"AWS Options"`
}

type AwsOptions struct {
	Profile string `short:"p" long:"profile" description:"AWS Profile to use. (If you are not using Vaulted)."`
	Region  string `short:"r" long:"region" description:"Region to target." default:"eu-west-1"`
}

type TargetOptions struct {
	Targets    []string `short:"t" long:"target" description:"One or more instance ids to target"`
	TargetFile string   `long:"target-file" description:"Path to a JSON file containing a list of targets."`
}
