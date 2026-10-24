// applyUnifiedDiff implements the stale-diff protection
// docs/RUNYX_CODE.md §2 requires for edit_file: every context and
// removed line in the diff must match the file's actual current
// content, or the whole edit is refused before anything is written.
// Scope is deliberately narrow for C0: one file, standard unified-diff
// hunks (@@ -l,c +l,c @@), no renames, no binary patches -- a model
// asking for more than that is a model asking for write_file instead.
package codeagent

import (
	"fmt"
	"strconv"
	"strings"
)

type diffHunk struct {
	origStart int
	lines     []diffLine // context (' '), removed ('-'), added ('+')
}

type diffLine struct {
	kind byte // ' ', '-', '+'
	text string
}

func applyUnifiedDiff(original, diff string) (string, error) {
	hunks, err := parseUnifiedDiff(diff)
	if err != nil {
		return "", err
	}
	if len(hunks) == 0 {
		return "", fmt.Errorf("diff contains no hunks")
	}

	origLines := splitLinesKeepEnding(original)
	var out []string
	cursor := 0 // next unconsumed index into origLines (0-based)

	for _, h := range hunks {
		start := h.origStart - 1 // 0-based
		if start < cursor {
			return "", fmt.Errorf("hunk at line %d overlaps a previous hunk", h.origStart)
		}
		if start > len(origLines) {
			return "", fmt.Errorf("hunk at line %d is past the end of the file (%d lines)", h.origStart, len(origLines))
		}
		// Copy everything between the previous hunk and this one untouched.
		out = append(out, origLines[cursor:start]...)
		cursor = start

		for _, l := range h.lines {
			switch l.kind {
			case ' ', '-':
				if cursor >= len(origLines) {
					return "", fmt.Errorf("diff context/removal past end of file at line %d: %q", cursor+1, l.text)
				}
				if stripEnding(origLines[cursor]) != l.text {
					return "", fmt.Errorf("diff does not match file content at line %d: expected %q, found %q (stale diff -- re-read the file and regenerate the diff)", cursor+1, l.text, stripEnding(origLines[cursor]))
				}
				if l.kind == ' ' {
					out = append(out, origLines[cursor])
				}
				cursor++
			case '+':
				out = append(out, l.text+"\n")
			default:
				return "", fmt.Errorf("unrecognized diff line prefix %q", l.kind)
			}
		}
	}
	out = append(out, origLines[cursor:]...)
	result := strings.Join(out, "")
	// Preserve "no trailing newline" exactly if the original had none
	// and the diff's final line wasn't an addition ending the file.
	if !strings.HasSuffix(original, "\n") && strings.HasSuffix(result, "\n") && len(out) > 0 {
		result = strings.TrimSuffix(result, "\n")
	}
	return result, nil
}

func parseUnifiedDiff(diff string) ([]diffHunk, error) {
	var hunks []diffHunk
	var cur *diffHunk
	// TrimSuffix first: Split's own trailing "" after a final "\n" is a
	// split artifact, not a genuine blank diff line (a genuine blank
	// context line in unified-diff format is " ", one space, not
	// truly empty).
	for _, raw := range strings.Split(strings.TrimSuffix(diff, "\n"), "\n") {
		switch {
		case strings.HasPrefix(raw, "--- ") || strings.HasPrefix(raw, "+++ "):
			continue // file headers: path is already known from the tool call
		case strings.HasPrefix(raw, "@@"):
			if cur != nil {
				hunks = append(hunks, *cur)
			}
			start, err := parseHunkHeader(raw)
			if err != nil {
				return nil, err
			}
			cur = &diffHunk{origStart: start}
		case raw == "":
			if cur != nil {
				cur.lines = append(cur.lines, diffLine{kind: ' ', text: ""})
			}
		default:
			if cur == nil {
				continue // ignore stray lines before the first hunk (e.g. "diff --git ...")
			}
			kind := raw[0]
			if kind != ' ' && kind != '-' && kind != '+' {
				return nil, fmt.Errorf("malformed diff line: %q", raw)
			}
			cur.lines = append(cur.lines, diffLine{kind: kind, text: raw[1:]})
		}
	}
	if cur != nil {
		hunks = append(hunks, *cur)
	}
	return hunks, nil
}

// parseHunkHeader extracts the original-file start line from
// "@@ -l,c +l,c @@" (the count fields aren't needed -- line-by-line
// context verification is the actual safety check).
func parseHunkHeader(header string) (int, error) {
	fields := strings.Fields(header)
	for _, f := range fields {
		if strings.HasPrefix(f, "-") {
			spec := strings.TrimPrefix(f, "-")
			spec = strings.SplitN(spec, ",", 2)[0]
			n, err := strconv.Atoi(spec)
			if err != nil {
				return 0, fmt.Errorf("malformed hunk header %q: %w", header, err)
			}
			return n, nil
		}
	}
	return 0, fmt.Errorf("malformed hunk header (no \"-\" field): %q", header)
}

func splitLinesKeepEnding(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.SplitAfter(s, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func stripEnding(s string) string {
	return strings.TrimSuffix(s, "\n")
}
