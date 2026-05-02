package client

import (
	"fmt"
	"strings"
)

// header is an ordered key/value pair used to build curl -H arguments.
type header struct {
	key, value string
}

// formatCurl returns a human-readable, copy-pasteable curl command that is
// equivalent to the described HTTP request.  The output uses a multi-line
// format with backslash continuations for readability.
//
// Single-quote shell escaping is applied to all values so the command can be
// pasted directly into bash/zsh.
func formatCurl(method, rawURL string, headers []header, body []byte, insecure bool) string {
	var sb strings.Builder

	sb.WriteString("curl -s")
	if insecure {
		sb.WriteString(" -k")
	}
	fmt.Fprintf(&sb, " -X %s", method)

	for _, h := range headers {
		fmt.Fprintf(&sb, " \\\n  -H %s", shellQuote(h.key+": "+h.value))
	}

	if len(body) > 0 {
		fmt.Fprintf(&sb, " \\\n  --data-raw %s", shellQuote(string(body)))
	}

	fmt.Fprintf(&sb, " \\\n  %s", shellQuote(rawURL))

	return sb.String()
}

// shellQuote wraps s in single quotes and escapes any literal single quotes
// within s using the '\"'\"' idiom, safe for bash and zsh.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
