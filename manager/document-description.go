package manager

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"strings"
)

// NewDocumentDescription creates a new Document from ssm.DocumentIdentifier.
func NewDocumentDescription(ssmDocument *ssm.DocumentDescription) *DocumentDescription {
	var parameters []*DocumentParameter

	for _, parameter := range ssmDocument.Parameters {
		parameters = append(parameters, &DocumentParameter{
			aws.StringValue(parameter.Name),
			aws.StringValue(parameter.Description),
			aws.StringValue(parameter.DefaultValue),
			aws.StringValue(parameter.Type),
		})
	}

	return &DocumentDescription{
		Name:            aws.StringValue(ssmDocument.Name),
		Description:     aws.StringValue(ssmDocument.Description),
		Owner:           aws.StringValue(ssmDocument.Owner),
		DocumentVersion: aws.StringValue(ssmDocument.DocumentVersion),
		DocumentFormat:  aws.StringValue(ssmDocument.DocumentFormat),
		DocumentType:    aws.StringValue(ssmDocument.DocumentType),
		SchemaVersion:   aws.StringValue(ssmDocument.SchemaVersion),
		TargetType:      aws.StringValue(ssmDocument.TargetType),
		Parameters:      parameters,
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
	Parameters      []*DocumentParameter
}

type DocumentParameter struct {
	Name         string
	Description  string
	DefaultValue string
	Type         string
}

// ParametersTabString returns all parameter values separated by "\t|\t" for
// an document. Use with tabwriter to output a table of parameters.
func (d *DocumentDescription) ParametersTabString() string {
	var del = "|"
	var tab = "\t"

	var newLine = "\n"
	var fields []string
	var line []string

	for _, parameter := range d.Parameters {
		line = []string{
			parameter.Name,
			parameter.Type,
			parameter.DefaultValue,
			parameter.Description,
		}
		fields = append(fields, strings.Join(line, tab+del+tab))
	}
	return strings.Join(fields, newLine)
}

// TabString returns all field values separated by "\t|\t" for
// an document. Use with tabwriter to output a table of documents.
func (d *DocumentDescription) TabString() string {
	var del = "|"
	var tab = "\t"

	fields := []string{
		d.Name,
		d.Description,
		d.Owner,
		d.DocumentVersion,
		d.DocumentFormat,
		d.DocumentType,
		d.SchemaVersion,
		d.TargetType,
	}
	return strings.Join(fields, tab+del+tab)
}
