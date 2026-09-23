package contextplan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/diffpal/lintpal/internal/apps/lintpal/git"
)

const surroundingLines = 2

type sourceKey struct {
	path string
	side git.Side
}

type hunkUnit struct {
	key   sourceKey
	hunk  int
	items []git.WorkItem
	text  string
}

// Assemble renders committed source into deterministic bounded hunk groups.
// It returns no groups when any item is invalid or an indivisible hunk is too large.
func Assemble(ctx context.Context, result git.Result, limits Limits) ([]Group, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	l, err := limits.normalized()
	if err != nil {
		return nil, err
	}
	stateCap := l.MaxStateBytes
	if l.ByteBudget < stateCap {
		stateCap = l.ByteBudget
	}
	if len(result.Items) > l.MaxItems {
		return nil, ErrLimit
	}
	items := append([]git.WorkItem(nil), result.Items...)
	sort.Slice(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Side != b.Side {
			return a.Side < b.Side
		}
		if a.Hunk != b.Hunk {
			return a.Hunk < b.Hunk
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		if a.EndLine != b.EndLine {
			return a.EndLine < b.EndLine
		}
		return a.ID < b.ID
	})
	seen := make(map[string]bool, len(items))
	sources := make(map[sourceKey][]string)
	units := make([]hunkUnit, 0, len(items))
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !validItem(item) || seen[item.ID] {
			return nil, ErrInvalidInput
		}
		seen[item.ID] = true
		key := sourceKey{item.Path, item.Side}
		lines, ok := sources[key]
		if !ok {
			source, err := result.Source(item)
			if err != nil || !utf8.Valid(source) {
				return nil, ErrInvalidInput
			}
			lines = splitLines(source)
			sources[key] = lines
		}
		if item.EndLine > len(lines) {
			return nil, ErrInvalidInput
		}
		if len(units) == 0 || units[len(units)-1].key != key || units[len(units)-1].hunk != item.Hunk {
			units = append(units, hunkUnit{key: key, hunk: item.Hunk})
		}
		unit := &units[len(units)-1]
		if len(unit.items) > 0 && item.StartLine <= unit.items[len(unit.items)-1].EndLine {
			return nil, ErrInvalidInput
		}
		unit.items = append(unit.items, item)
	}
	for i := range units {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		text, err := renderUnit(ctx, units[i], sources[units[i].key], stateCap)
		if err != nil {
			return nil, err
		}
		units[i].text = text
	}
	groups := make([]Group, 0, len(units))
	totalState := 0
	for _, unit := range units {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(unit.text) > l.MaxTotalStateBytes-totalState {
			return nil, ErrLimit
		}
		if len(groups) > 0 {
			last := &groups[len(groups)-1]
			first := last.Items[0]
			if first.Path == unit.key.path && first.Side == unit.key.side &&
				len(last.State)+len(unit.text) <= stateCap {
				last.State += unit.text
				totalState += len(unit.text)
				last.Items = append(last.Items, unit.items...)
				last.ID = groupID(last.Items)
				continue
			}
		}
		if len(groups) >= l.MaxGroups {
			return nil, ErrLimit
		}
		groups = append(groups, Group{ID: groupID(unit.items), State: unit.text, Items: append([]git.WorkItem(nil), unit.items...)})
		totalState += len(unit.text)
	}
	return groups, nil
}

func validItem(item git.WorkItem) bool {
	if len(item.ID) != 64 || item.Path == "" || item.Hunk < 1 || item.StartLine < 1 || item.EndLine < item.StartLine {
		return false
	}
	if _, err := hex.DecodeString(item.ID); err != nil {
		return false
	}
	if item.Side == git.Left {
		return item.Path == item.OldPath
	}
	if item.Side == git.Right {
		return item.Path == item.NewPath
	}
	return false
}

func splitLines(source []byte) []string {
	if len(source) == 0 {
		return nil
	}
	lines := strings.Split(string(source), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func renderUnit(ctx context.Context, unit hunkUnit, lines []string, maxBytes int) (string, error) {
	var b strings.Builder
	if err := writeBounded(&b, "path="+strconv.Quote(unit.key.path)+"\nside="+string(unit.key.side)+"\nhunk="+strconv.Itoa(unit.hunk)+"\n", maxBytes); err != nil {
		return "", err
	}
	for _, item := range unit.items {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		start := item.StartLine - surroundingLines
		if start < 1 {
			start = 1
		}
		end := item.EndLine + surroundingLines
		if end > len(lines) {
			end = len(lines)
		}
		if err := writeBounded(&b, "span="+strconv.Itoa(item.StartLine)+"-"+strconv.Itoa(item.EndLine)+"\n", maxBytes); err != nil {
			return "", err
		}
		for number := start; number <= end; number++ {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			line := strconv.Itoa(number) + ":" + strconv.Quote(lines[number-1]) + "\n"
			if err := writeBounded(&b, line, maxBytes); err != nil {
				return "", err
			}
		}
	}
	return b.String(), nil
}

func writeBounded(b *strings.Builder, part string, limit int) error {
	if len(part) > limit-b.Len() {
		return ErrLimit
	}
	b.WriteString(part)
	return nil
}

func groupID(items []git.WorkItem) string {
	h := sha256.New()
	for _, item := range items {
		h.Write([]byte(item.ID))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
