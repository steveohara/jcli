// Package output provides formatting helpers for printing Jira API responses
// in table, JSON, or plain-text form.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
)

// Format constants for the --output flag.
const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatPlain = "plain"
)

// Printer formats and writes data to an io.Writer.
type Printer struct {
	w      io.Writer
	format string
}

// New returns a new Printer writing to w in the given format.
func New(w io.Writer, format string) *Printer {
	if format == "" {
		format = FormatTable
	}
	return &Printer{w: w, format: format}
}

// Default returns a Printer writing to os.Stdout using the given format.
func Default(format string) *Printer {
	return New(os.Stdout, format)
}

// JSON prints v as indented JSON regardless of the configured format.
func (p *Printer) JSON(v interface{}) error {
	enc := json.NewEncoder(p.w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Table prints rows as an ASCII table.  headers is the column header row.
func (p *Printer) Table(headers []string, rows [][]string) {
	switch p.format {
	case FormatJSON:
		// Convert to slice of maps for JSON output
		var out []map[string]string
		for _, row := range rows {
			m := make(map[string]string, len(headers))
			for i, h := range headers {
				if i < len(row) {
					m[strings.ToLower(strings.ReplaceAll(h, " ", "_"))] = row[i]
				}
			}
			out = append(out, m)
		}
		_ = p.JSON(out)
	case FormatPlain:
		for _, row := range rows {
			fmt.Fprintln(p.w, strings.Join(row, "\t"))
		}
	default:
		table := tablewriter.NewWriter(p.w)
		// Convert string headers to []any for the new API
		hdr := make([]any, len(headers))
		for i, h := range headers {
			hdr[i] = h
		}
		table.Header(hdr...)
		for _, row := range rows {
			r := make([]any, len(row))
			for i, v := range row {
				r[i] = v
			}
			_ = table.Append(r...)
		}
		_ = table.Render()
	}
}

// KV prints a key-value map in a two-column table.
func (p *Printer) KV(pairs [][]string) {
	switch p.format {
	case FormatJSON:
		m := make(map[string]string, len(pairs))
		for _, pair := range pairs {
			if len(pair) == 2 {
				m[strings.ToLower(strings.ReplaceAll(pair[0], " ", "_"))] = pair[1]
			}
		}
		_ = p.JSON(m)
	case FormatPlain:
		for _, pair := range pairs {
			if len(pair) == 2 {
				fmt.Fprintf(p.w, "%s: %s\n", pair[0], pair[1])
			}
		}
	default:
		table := tablewriter.NewWriter(p.w)
		for _, pair := range pairs {
			if len(pair) == 2 {
				_ = table.Append(pair[0], pair[1])
			}
		}
		_ = table.Render()
	}
}

// Success prints a success message to stderr.
func Success(msg string) {
	fmt.Fprintln(os.Stderr, "✓ "+msg)
}

// Stdout returns os.Stdout for use in command output formatting.
func Stdout() *os.File {
	return os.Stdout
}

// Truncate shortens s to at most n runes, appending "…" if truncated.
func Truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n-1]) + "…"
}

