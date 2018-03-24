package manager

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"strings"
)

// NewDocumentDescription creates a new Document from ssm.DocumentIdentifier.
func NewDocumentDescription(ssmDocument *ssm.DocumentDescription) *DocumentDescription {
	return &DocumentDescription{
		Name:            aws.StringValue(ssmDocument.Name),
		Description:     aws.StringValue(ssmDocument.Description),
		Owner:           aws.StringValue(ssmDocument.Owner),
		DocumentVersion: aws.StringValue(ssmDocument.DocumentVersion),
		DocumentFormat:  aws.StringValue(ssmDocument.DocumentFormat),
		DocumentType:    aws.StringValue(ssmDocument.DocumentType),
		SchemaVersion:   aws.StringValue(ssmDocument.SchemaVersion),
		TargetType:      aws.StringValue(ssmDocument.TargetType),
		Parameters:      ssmDocument.Parameters,
	}
}

// DocumentDescription describes relevant information about a SSM Document
type DocumentDescription struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	Owner           string `json:"owner"`
	DocumentVersion string `json:"documentVersion"`
	DocumentFormat  string `json:"documentFormat"`
	DocumentType    string `json:"documentType"`
	SchemaVersion   string `json:"schemaVersion"`
	TargetType      string `json:"targetType"`
	Parameters      []*ssm.DocumentParameter
}

// ParametersString returns all field values for parameters, should be converted to a tabwriter
func (d *DocumentDescription) ParametersString() string {

	var newLine = "\n"
	var fields []string

	for _, parameter := range d.Parameters {
		fields = append(fields, fmt.Sprintf("Name: %s", aws.StringValue(parameter.Name)))
		fields = append(fields, fmt.Sprintf("Description: %s", aws.StringValue(parameter.Description)))
		fields = append(fields, fmt.Sprintf("DefaultValue: %s", aws.StringValue(parameter.DefaultValue)))
		fields = append(fields, fmt.Sprintf("Type: %s\n", aws.StringValue(parameter.Type)))
	}

	return strings.Join(fields, newLine)
}
