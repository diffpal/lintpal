package git

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var ErrMalformedDiff = errors.New("malformed Git diff")

const maxDiffLineBytes = 1 << 20
const maxGitPathBytes = 4 << 10

// Side identifies the committed file whose changed lines carry the anchor.
type Side string

const (
	Left  Side = "LEFT"
	Right Side = "RIGHT"
)

type rawFile struct {
	oldPath string
	newPath string
	oldMode string
	newMode string
	status  byte
}

type changedSpan struct {
	side  Side
	start int
	end   int
	hunk  int
}

type fileChange struct {
	file   rawFile
	spans  []changedSpan
	binary bool
}

var hunkHeader = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(?:.*)$`)

func parseRaw(data []byte) ([]rawFile, error) {
	if len(data) == 0 {
		return nil, nil
	}
	if data[len(data)-1] != 0 {
		return nil, ErrMalformedDiff
	}
	fields := bytes.Split(data[:len(data)-1], []byte{0})
	files := make([]rawFile, 0, len(fields)/2)
	for i := 0; i < len(fields); {
		meta := strings.Fields(string(fields[i]))
		i++
		if len(meta) != 5 || !strings.HasPrefix(meta[0], ":") || len(meta[0]) != 7 || len(meta[4]) == 0 {
			return nil, ErrMalformedDiff
		}
		if !validMode(meta[0][1:]) || !validMode(meta[1]) || !validRawStatus(meta[4]) {
			return nil, ErrMalformedDiff
		}
		if i >= len(fields) || len(fields[i]) == 0 {
			return nil, ErrMalformedDiff
		}
		f := rawFile{oldMode: meta[0][1:], newMode: meta[1], status: meta[4][0]}
		path := string(fields[i])
		if len(path) > maxGitPathBytes {
			return nil, ErrLimit
		}
		i++
		switch f.status {
		case 'A':
			f.newPath = path
		case 'D':
			f.oldPath = path
		case 'M', 'T':
			f.oldPath, f.newPath = path, path
		case 'R', 'C':
			if i >= len(fields) || len(fields[i]) == 0 {
				return nil, ErrMalformedDiff
			}
			f.oldPath, f.newPath = path, string(fields[i])
			if len(f.newPath) > maxGitPathBytes {
				return nil, ErrLimit
			}
			i++
		default:
			return nil, ErrMalformedDiff
		}
		files = append(files, f)
	}
	return files, nil
}

func validMode(mode string) bool {
	if len(mode) != 6 {
		return false
	}
	for _, digit := range mode {
		if digit < '0' || digit > '7' {
			return false
		}
	}
	return true
}

func validRawStatus(status string) bool {
	if len(status) == 0 {
		return false
	}
	switch status[0] {
	case 'A', 'D', 'T':
		return len(status) == 1
	case 'M', 'R', 'C':
		if len(status) == 1 {
			return status[0] == 'M'
		}
		if len(status) < 2 || len(status) > 4 {
			return false
		}
		for _, digit := range status[1:] {
			if digit < '0' || digit > '9' {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func parsePatch(data []byte, raw []rawFile) ([]fileChange, error) {
	if len(data) == 0 {
		if len(raw) == 0 {
			return nil, nil
		}
		return nil, ErrMalformedDiff
	}
	if data[len(data)-1] != '\n' {
		return nil, ErrMalformedDiff
	}
	lines := bytes.Split(data[:len(data)-1], []byte{'\n'})
	for _, line := range lines {
		if len(line) > maxDiffLineBytes {
			return nil, ErrLimit
		}
	}
	changes := make([]fileChange, 0, len(raw))
	for i := 0; i < len(lines); {
		if !bytes.HasPrefix(lines[i], []byte("diff --git ")) || len(changes) >= len(raw) {
			return nil, ErrMalformedDiff
		}
		change := fileChange{file: raw[len(changes)]}
		i++
		oldMarker, newMarker := false, false
		hunkOrdinal := 0
		for i < len(lines) && !bytes.HasPrefix(lines[i], []byte("diff --git ")) {
			line := string(lines[i])
			switch {
			case strings.HasPrefix(line, "@@ "):
				if !oldMarker || !newMarker || change.binary {
					return nil, ErrMalformedDiff
				}
				hunkOrdinal++
				spans, next, err := parseHunk(lines, i, hunkOrdinal)
				if err != nil {
					return nil, err
				}
				change.spans = append(change.spans, spans...)
				i = next
				continue
			case strings.HasPrefix(line, "--- "):
				if oldMarker || !samePatchPath(strings.TrimPrefix(line, "--- "), change.file.oldPath, "a/") {
					return nil, ErrMalformedDiff
				}
				oldMarker = true
			case strings.HasPrefix(line, "+++ "):
				if newMarker || !oldMarker || !samePatchPath(strings.TrimPrefix(line, "+++ "), change.file.newPath, "b/") {
					return nil, ErrMalformedDiff
				}
				newMarker = true
			case strings.HasPrefix(line, "Binary files "), line == "GIT binary patch":
				if oldMarker || newMarker || hunkOrdinal > 0 {
					return nil, ErrMalformedDiff
				}
				change.binary = true
			case strings.HasPrefix(line, "index "), strings.HasPrefix(line, "new file mode "),
				strings.HasPrefix(line, "deleted file mode "), strings.HasPrefix(line, "old mode "),
				strings.HasPrefix(line, "new mode "), strings.HasPrefix(line, "similarity index "),
				strings.HasPrefix(line, "dissimilarity index "), strings.HasPrefix(line, "rename from "),
				strings.HasPrefix(line, "rename to "), strings.HasPrefix(line, "copy from "),
				strings.HasPrefix(line, "copy to "):
				// Metadata is authoritative in the NUL-delimited raw record.
			default:
				return nil, ErrMalformedDiff
			}
			i++
		}
		if oldMarker != newMarker || (hunkOrdinal > 0 && (!oldMarker || !newMarker)) {
			return nil, ErrMalformedDiff
		}
		changes = append(changes, change)
	}
	if len(changes) != len(raw) {
		return nil, ErrMalformedDiff
	}
	return changes, nil
}

func samePatchPath(marker, rawPath, prefix string) bool {
	marker = strings.TrimSuffix(marker, "\t")
	if rawPath == "" {
		return marker == "/dev/null"
	}
	if len(marker) > 0 && marker[0] == '"' {
		unquoted, err := strconv.Unquote(marker)
		if err != nil {
			return false
		}
		marker = unquoted
	}
	return marker == prefix+rawPath
}

func parseHunk(lines [][]byte, index, ordinal int) ([]changedSpan, int, error) {
	match := hunkHeader.FindStringSubmatch(string(lines[index]))
	if match == nil {
		return nil, 0, ErrMalformedDiff
	}
	oldStart, err := strconv.Atoi(match[1])
	if err != nil {
		return nil, 0, ErrMalformedDiff
	}
	oldCount, err := hunkCount(match[2])
	if err != nil {
		return nil, 0, err
	}
	newStart, err := strconv.Atoi(match[3])
	if err != nil {
		return nil, 0, ErrMalformedDiff
	}
	newCount, err := hunkCount(match[4])
	maxInt := int(^uint(0) >> 1)
	if err != nil || oldStart < 0 || newStart < 0 || (oldCount > 0 && oldStart == 0) ||
		(newCount > 0 && newStart == 0) || oldCount > maxInt-oldStart || newCount > maxInt-newStart {
		return nil, 0, ErrMalformedDiff
	}
	oldLine, newLine := oldStart, newStart
	var spans []changedSpan
	lastContent := false
	i := index + 1
	for i < len(lines) && !bytes.HasPrefix(lines[i], []byte("diff --git ")) && !bytes.HasPrefix(lines[i], []byte("@@ ")) {
		line := lines[i]
		if len(line) == 0 {
			return nil, 0, ErrMalformedDiff
		}
		switch line[0] {
		case ' ':
			oldLine++
			newLine++
			lastContent = true
		case '-':
			spans = appendChanged(spans, Left, oldLine, ordinal)
			oldLine++
			lastContent = true
		case '+':
			spans = appendChanged(spans, Right, newLine, ordinal)
			newLine++
			lastContent = true
		case '\\':
			if !lastContent || string(line) != "\\ No newline at end of file" {
				return nil, 0, ErrMalformedDiff
			}
			lastContent = false
		default:
			return nil, 0, ErrMalformedDiff
		}
		if oldLine > oldStart+oldCount || newLine > newStart+newCount {
			return nil, 0, ErrMalformedDiff
		}
		i++
	}
	if oldLine != oldStart+oldCount || newLine != newStart+newCount {
		return nil, 0, fmt.Errorf("hunk line count: %w", ErrMalformedDiff)
	}
	return spans, i, nil
}

func hunkCount(text string) (int, error) {
	if text == "" {
		return 1, nil
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < 0 {
		return 0, ErrMalformedDiff
	}
	return n, nil
}

func appendChanged(spans []changedSpan, side Side, line, hunk int) []changedSpan {
	if line <= 0 {
		return spans
	}
	if n := len(spans); n > 0 && spans[n-1].side == side && spans[n-1].end+1 == line && spans[n-1].hunk == hunk {
		spans[n-1].end = line
		return spans
	}
	return append(spans, changedSpan{side: side, start: line, end: line, hunk: hunk})
}
