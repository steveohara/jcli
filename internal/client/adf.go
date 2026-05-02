package client

import (
	"encoding/json"
	"strings"
)

// ----------------------------------------------------------------------------
// Atlassian Document Format (ADF) — response decoding
//
// Jira REST API v2 uses plain strings for rich-text fields (issue description,
// comment body, worklog comment).  However, some server responses may still
// include ADF objects (e.g. when data was originally written via the v3 API).
// This file provides ADFString, a string type with a custom JSON unmarshaler
// that accepts either a plain string or a full ADF document and exposes the
// extracted plain text.
//
// Reference: https://developer.atlassian.com/cloud/jira/platform/apis/document/structure/
// ----------------------------------------------------------------------------

// ADFString is a string value that may be unmarshaled from either a JSON
// string or an Atlassian Document Format object.  In both cases the plain-text
// content is stored in the string value.
type ADFString string

// UnmarshalJSON implements json.Unmarshaler.  It handles three cases:
//  1. A JSON string  — stored as-is.
//  2. An ADF document object — plain text is extracted recursively.
//  3. JSON null — left as empty string.
func (a *ADFString) UnmarshalJSON(data []byte) error {
	// Case 1: plain JSON string (API v2 compat or simple fields).
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*a = ADFString(s)
		return nil
	}

	// Case 2: ADF object.
	var doc adfNode
	if err := json.Unmarshal(data, &doc); err == nil {
		*a = ADFString(adfNodeText(&doc))
		return nil
	}

	// Case 3: null or unrecognised — leave empty.
	return nil
}

// adfNode mirrors the recursive ADF node structure.
type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
	// Marks are intentionally ignored — we only need plain text.
}

// adfNodeText recursively extracts plain text from an ADF node tree.  Block-
// level node types are separated by newlines; inline nodes are concatenated.
func adfNodeText(n *adfNode) string {
	// Leaf text node.
	if n.Type == "text" {
		return n.Text
	}
	if n.Type == "hardBreak" {
		return "\n"
	}

	var sb strings.Builder
	for i, child := range n.Content {
		sb.WriteString(adfNodeText(&child))
		// Insert a newline between top-level block elements so paragraphs,
		// list items, headings, etc. are separated.
		if isBlockNode(child.Type) && i < len(n.Content)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// isBlockNode reports whether the ADF node type is a block-level element.
func isBlockNode(t string) bool {
	switch t {
	case "paragraph", "heading", "bulletList", "orderedList",
		"listItem", "blockquote", "codeBlock", "rule", "panel",
		"table", "tableRow", "tableCell", "tableHeader", "expand":
		return true
	}
	return false
}
