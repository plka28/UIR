package utils

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// CSVRow keeps normalized CSV values with their source path and line number.
type CSVRow struct {
	Path   string
	Line   int
	Values map[string]string
}

// ReadCSVRows reads CSV data with normalized headers and useful error context.
func ReadCSVRows(path string, requiredColumns []string) ([]CSVRow, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	headers, seen, err := normalizeCSVHeader(path, header)
	if err != nil {
		return nil, err
	}
	if err := requireCSVColumns(path, seen, requiredColumns); err != nil {
		return nil, err
	}

	var rows []CSVRow
	line := 1
	for {
		line++
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, line, err)
		}
		if isEmptyCSVRecord(record) {
			continue
		}
		rows = append(rows, CSVRow{
			Path:   path,
			Line:   line,
			Values: csvRecordValues(headers, record),
		})
	}

	return rows, nil
}

// RequiredValue returns the first non-empty value among alternate column names.
func (row CSVRow) RequiredValue(names ...string) (string, error) {
	for _, name := range names {
		value, ok := row.Values[NormalizeHeader(name)]
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("%s:%d: empty %s", row.Path, row.Line, name)
		}
		return value, nil
	}
	return "", fmt.Errorf("%s:%d: missing required column: %s", row.Path, row.Line, strings.Join(names, " or "))
}

// RequiredFloat parses the first matching required value as a flexible float.
func (row CSVRow) RequiredFloat(names ...string) (float64, error) {
	value, err := row.RequiredValue(names...)
	if err != nil {
		return 0, err
	}
	parsed, err := ParseFlexibleFloat(value)
	if err != nil {
		return 0, fmt.Errorf("%s:%d: invalid numeric value %q: %w", row.Path, row.Line, value, err)
	}
	return parsed, nil
}

func normalizeCSVHeader(path string, header []string) ([]string, map[string]struct{}, error) {
	headers := make([]string, len(header))
	seen := make(map[string]struct{}, len(header))
	for index, rawHeader := range header {
		name := NormalizeHeader(rawHeader)
		headers[index] = name
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			return nil, nil, fmt.Errorf("%s: duplicate column %q", path, name)
		}
		seen[name] = struct{}{}
	}
	return headers, seen, nil
}

func requireCSVColumns(path string, headers map[string]struct{}, required []string) error {
	var missing []string
	for _, column := range required {
		if _, ok := headers[NormalizeHeader(column)]; !ok {
			missing = append(missing, column)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s: missing required columns: %s", path, strings.Join(missing, ", "))
	}
	return nil
}

func csvRecordValues(headers []string, record []string) map[string]string {
	values := make(map[string]string, len(headers))
	for index, name := range headers {
		if name == "" {
			continue
		}
		if index < len(record) {
			values[name] = strings.TrimSpace(record[index])
		} else {
			values[name] = ""
		}
	}
	return values
}

func isEmptyCSVRecord(record []string) bool {
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}
