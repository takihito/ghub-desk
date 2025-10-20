package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// OutputFormat represents the supported rendering format for view results.
type OutputFormat string

const (
	// FormatTable renders output as tab-separated text tables (default).
	FormatTable OutputFormat = "table"
	// FormatJSON renders output as JSON.
	FormatJSON OutputFormat = "json"
	// FormatYAML renders output as YAML.
	FormatYAML OutputFormat = "yaml"
)

// ViewOptions controls how HandleViewTarget renders results.
type ViewOptions struct {
	Format OutputFormat
}

// ParseOutputFormat converts a raw string into an OutputFormat, defaulting to table.
func ParseOutputFormat(raw string) (OutputFormat, error) {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return FormatTable, nil
	}
	switch trimmed {
	case string(FormatTable):
		return FormatTable, nil
	case string(FormatJSON):
		return FormatJSON, nil
	case string(FormatYAML):
		return FormatYAML, nil
	default:
		return "", fmt.Errorf("unsupported format: %s", raw)
	}
}

func (o ViewOptions) formatOrDefault() OutputFormat {
	if o.Format == "" {
		return FormatTable
	}
	return o.Format
}

func renderByFormat(format OutputFormat, tableFn func() error, payload interface{}) error {
	switch format {
	case FormatTable:
		if tableFn == nil {
			return nil
		}
		return tableFn()
	case FormatJSON:
		return printJSON(payload)
	case FormatYAML:
		return printYAML(payload)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

func printJSON(payload interface{}) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func printYAML(payload interface{}) error {
	data, err := yaml.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	fmt.Print(string(data))
	return nil
}
