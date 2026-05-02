package client

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// Compiled once at package init.
var (
	// Blocks whose content should be discarded entirely.
	reNoContent = regexp.MustCompile(`(?is)<(script|style|head)[^>]*>.*?</(script|style|head)>`)

	// Anchor tags – strip the entire element; link text is navigation noise.
	reAnchor = regexp.MustCompile(`(?is)<a[^>]*>.*?</a>`)

	// Named tags we promote to their own labelled lines.
	reTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reH     = regexp.MustCompile(`(?is)<h[1-3][^>]*>(.*?)</h[1-3]>`)

	// Block-level elements that act as natural line breaks.
	reBlock = regexp.MustCompile(`(?i)</?(?:p|div|li|br|tr|dt|dd|blockquote|pre|hr)[^>]*>`)

	// All remaining tags.
	reTag = regexp.MustCompile(`<[^>]+>`)

	// Runs of whitespace (including newlines produced above).
	reSpace = regexp.MustCompile(`[[:space:]]+`)
)

// htmlToText converts an HTML document to a compact, human-readable plain-text
// string suitable for display in a CLI.  It:
//
//   - Discards <script>, <style> and <head> blocks entirely.
//   - Discards <a> anchor elements (link text is navigation noise in error pages).
//   - Promotes <title> to a prominent first line.
//   - Promotes <h1>–<h3> headings to their own lines.
//   - Inserts line breaks at block-level elements (p, div, li, br, …).
//   - Strips all remaining tags.
//   - Decodes HTML entities (&amp; → &, &#39; → ', …).
//   - Drops orphaned labels (lines that end with ':' and are ≤ 20 chars).
//   - Collapses whitespace and drops blank / duplicate lines.
//   - Caps the output at maxLines non-empty lines to stay CLI-friendly.
func htmlToText(src []byte, maxLines int) string {
	s := string(src)

	// 1. Capture the page title first (before we remove <head>).
	var title string
	if m := reTitle.FindStringSubmatch(s); len(m) == 2 {
		title = strings.TrimSpace(reTag.ReplaceAllString(m[1], " "))
		title = html.UnescapeString(reSpace.ReplaceAllString(title, " "))
	}

	// 2. Remove no-content blocks.
	s = reNoContent.ReplaceAllString(s, " ")

	// 3. Strip anchor elements – their text is navigation noise in error pages.
	s = reAnchor.ReplaceAllString(s, " ")

	// 4. Promote headings to their own lines.
	s = reH.ReplaceAllStringFunc(s, func(h string) string {
		inner := reH.FindStringSubmatch(h)
		if len(inner) < 2 {
			return "\n"
		}
		text := strings.TrimSpace(reTag.ReplaceAllString(inner[1], " "))
		return "\n" + text + "\n"
	})

	// 5. Turn block-level elements into newlines.
	s = reBlock.ReplaceAllString(s, "\n")

	// 6. Strip remaining tags.
	s = reTag.ReplaceAllString(s, "")

	// 7. Decode HTML entities.
	s = html.UnescapeString(s)

	// 8. Split into lines, clean each one, collect non-empty.
	raw := strings.Split(s, "\n")
	var lines []string
	seen := map[string]bool{}
	for _, line := range raw {
		// Collapse inline whitespace.
		line = strings.TrimFunc(reSpace.ReplaceAllString(line, " "), unicode.IsSpace)
		if line == "" || seen[line] {
			continue
		}
		// Skip lines that look like leftover markup artefacts.
		if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "//") {
			continue
		}
		// Skip orphaned section labels: short lines whose only content is a
		// word followed by a colon, e.g. "Warning:" or "Note:" left behind
		// when the surrounding block element was stripped.
		if len(line) <= 20 && strings.HasSuffix(line, ":") && !strings.ContainsAny(line, " \t") {
			continue
		}
		seen[line] = true
		lines = append(lines, line)
		if len(lines) == maxLines {
			break
		}
	}

	// 9. Assemble output: title first (if distinct from body), then body lines.
	var out []string
	if title != "" {
		out = append(out, title)
	}
	for _, l := range lines {
		if l == title {
			continue // already shown
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// isHTML returns true when the Content-Type header or the first bytes of the
// body indicate an HTML document.
func isHTML(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	// Sniff the first 512 bytes.
	sniff := strings.ToLower(strings.TrimSpace(string(body[:min512(body)])))
	return strings.HasPrefix(sniff, "<!doctype html") ||
		strings.HasPrefix(sniff, "<html")
}

// min512 returns the length of b capped at 512, used to safely slice a byte
// slice when sniffing the first few bytes for an HTML signature.
func min512(b []byte) int {
	if len(b) < 512 {
		return len(b)
	}
	return 512
}
