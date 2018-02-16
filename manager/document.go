package manager

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"strings"
)

// NewInstance creates a new Document from ssm.DocumentIdentifier.
func NewDocument(ssmDocument *ssm.DocumentIdentifier) *Document {
	return &Document{
		Name:            aws.StringValue(ssmDocument.Name),
		Owner:           aws.StringValue(ssmDocument.Owner),
		DocumentVersion: aws.StringValue(ssmDocument.DocumentVersion),
		DocumentFormat:  aws.StringValue(ssmDocument.DocumentFormat),
		DocumentType:    aws.StringValue(ssmDocument.DocumentType),
		SchemaVersion:   aws.StringValue(ssmDocument.SchemaVersion),
		TargetType:      aws.StringValue(ssmDocument.TargetType),
	}
}

// Document describes relevant information about a SSM Document
type Document struct {
	Name            string `json:"name"`
	Owner           string `json:"owner"`
	DocumentVersion string `json:"documentVersion"`
	DocumentFormat  string `json:"documentFormat"`
	DocumentType    string `json:"documentType"`
	SchemaVersion   string `json:"schemaVersion"`
	TargetType      string `json:"targetType"`
}

// TabString returns all field values separated by "\t|\t" for
// an document. Use with tabwriter to output a table of documents.
func (d *Document) TabString() string {
	var del = "|"
	var tab = "\t"

	fields := []string{
		d.Name,
		d.Owner,
		d.DocumentVersion,
		d.DocumentFormat,
		d.DocumentType,
		d.SchemaVersion,
		d.TargetType,
	}
	return strings.Join(fields, tab+del+tab)
}
