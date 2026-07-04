package ymdb

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	PostFormatText = "text"
	PostFormatJSON = "json"
	PostFormatCSV  = "csv"
)

const (
	PostMimeText     = "text"
	PostMimeHTML     = "html"
	PostMimeMarkdown = "markdown"
)

func validPostFormat(format string) bool {
	switch format {
	case PostFormatText, PostFormatJSON, PostFormatCSV:
		return true
	default:
		return false
	}
}

// Validate checks the Post fields whose values form part of YMDB's storage contract.
func (p *Post) Validate() error {
	if p == nil {
		return errors.New("ymdb: post is nil")
	}
	if !validPostFormat(p.Format) {
		return fmt.Errorf("ymdb: unsupported post data format %q", p.Format)
	}
	if strings.TrimSpace(p.Mime) == "" {
		return errors.New("ymdb: post content MIME is required")
	}
	if p.Format == PostFormatJSON && !json.Valid([]byte(p.Data)) {
		return errors.New("ymdb: post data is not valid JSON")
	}
	if p.Format == PostFormatCSV {
		reader := csv.NewReader(strings.NewReader(p.Data))
		if _, err := reader.ReadAll(); err != nil {
			return fmt.Errorf("ymdb: post data is not valid CSV: %w", err)
		}
	}
	return nil
}

func (p *Post) SetContent(content, mime string) error {
	mime = strings.ToLower(strings.TrimSpace(mime))
	if mime == "" {
		return errors.New("ymdb: post content MIME is required")
	}
	p.Content, p.Mime = content, mime
	return nil
}

func (p *Post) SetTextData(value string) { p.Data, p.Format = value, PostFormatText }

func (p *Post) SetJSONData(value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("ymdb: encode post JSON data: %w", err)
	}
	p.Data, p.Format = string(encoded), PostFormatJSON
	return nil
}

func (p *Post) DecodeJSONData(destination any) error {
	if p.Format != PostFormatJSON {
		return fmt.Errorf("ymdb: post data format is %q, not json", p.Format)
	}
	if destination == nil {
		return errors.New("ymdb: JSON destination is required")
	}
	return json.Unmarshal([]byte(p.Data), destination)
}

func (p *Post) SetCSVData(records [][]string) error {
	var output strings.Builder
	writer := csv.NewWriter(&output)
	writer.WriteAll(records)
	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("ymdb: encode post CSV data: %w", err)
	}
	p.Data, p.Format = output.String(), PostFormatCSV
	return nil
}

func (p *Post) DecodeCSVData() ([][]string, error) {
	if p.Format != PostFormatCSV {
		return nil, fmt.Errorf("ymdb: post data format is %q, not csv", p.Format)
	}
	records, err := csv.NewReader(strings.NewReader(p.Data)).ReadAll()
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("ymdb: decode post CSV data: %w", err)
	}
	return records, nil
}
