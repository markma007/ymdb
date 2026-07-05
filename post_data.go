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
	PostFormatText    = "text"
	PostFormatJSON    = "json"
	PostFormatCSV     = "csv"
	PostFormatTabular = "tabular" // PostFormatTabular stores comma-separated cells and semicolon-separated rows.
)

const (
	PostMimeText     = "text"
	PostMimeHTML     = "html"
	PostMimeMarkdown = "markdown"
	PostMimeXML      = "xml"
)

func validPostFormat(format string) bool {
	switch format {
	case PostFormatText, PostFormatJSON, PostFormatCSV, PostFormatTabular:
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
	if p.Format == PostFormatTabular {
		if _, err := decodeTabular(p.Data); err != nil {
			return fmt.Errorf("ymdb: post data is not valid tabular data: %w", err)
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

// SetTabularData stores records as comma-separated cells and
// semicolon-separated rows, for example "a1,a2;b1,b2". Delimiters and quotes
// inside cells are escaped using CSV-style double quoting.
func (p *Post) SetTabularData(records [][]string) {
	rows := make([]string, len(records))
	for i, record := range records {
		cells := make([]string, len(record))
		for j, cell := range record {
			cells[j] = encodeTabularCell(cell)
		}
		rows[i] = strings.Join(cells, ",")
	}
	p.Data, p.Format = strings.Join(rows, ";"), PostFormatTabular
}

// DecodeTabularData decodes comma-separated cells and semicolon-separated rows.
func (p *Post) DecodeTabularData() ([][]string, error) {
	if p.Format != PostFormatTabular {
		return nil, fmt.Errorf("ymdb: post data format is %q, not tabular", p.Format)
	}
	return decodeTabular(p.Data)
}

func encodeTabularCell(cell string) string {
	if !strings.ContainsAny(cell, ",;\"\r\n") {
		return cell
	}
	return `"` + strings.ReplaceAll(cell, `"`, `""`) + `"`
}

func decodeTabular(data string) ([][]string, error) {
	if data == "" {
		return [][]string{}, nil
	}
	var csvData strings.Builder
	inQuotes := false
	for i := 0; i < len(data); i++ {
		char := data[i]
		if char == '"' {
			csvData.WriteByte(char)
			if inQuotes && i+1 < len(data) && data[i+1] == '"' {
				csvData.WriteByte(data[i+1])
				i++
				continue
			}
			inQuotes = !inQuotes
			continue
		}
		if char == ';' && !inQuotes {
			csvData.WriteByte('\n')
		} else {
			csvData.WriteByte(char)
		}
	}
	records, err := csv.NewReader(strings.NewReader(csvData.String())).ReadAll()
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("ymdb: decode post tabular data: %w", err)
	}
	return records, nil
}
