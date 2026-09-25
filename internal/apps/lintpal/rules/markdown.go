package rules

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

const maxMarkdownFileBytes = 8 << 10
const maxMarkdownPackBytes = 256 << 10
const maxMarkdownEntries = 4096
const maxMarkdownIDBytes = 63 // 64-byte work item ID + '/' + rule ID <= Jev's 128-byte ID limit.

// Mandate is one freeform Markdown requirement. ID is its path relative to a rule root.
type Mandate struct {
	ID   string
	Body string
}

type mandateMetadata struct {
	Severity  *Severity `yaml:"severity"`
	Threshold *float64  `yaml:"threshold"`
	Title     *string   `yaml:"title"`
}

// LoadDirectory reads Markdown mandates under root without following symlinks.
// Invalid or over-limit input returns no partial pack.
func LoadDirectory(ctx context.Context, root string) (Pack, error) {
	mandates, err := ReadMandates(ctx, root)
	if err != nil {
		return Pack{}, err
	}
	return CompileMandates(mandates)
}

// ReadMandates returns bounded, validated Markdown files in stable path order.
func ReadMandates(ctx context.Context, root string) ([]Mandate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidPack
	}
	confined, err := os.OpenRoot(root)
	if err != nil {
		return nil, ErrInvalidPack
	}
	defer func() { _ = confined.Close() }()
	var mandates []Mandate
	entries, total := 0, 0
	err = filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			return ErrInvalidPack
		}
		entries++
		if entries > maxMarkdownEntries || len(mandates) > maxRules {
			return ErrPackLimit
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return ErrInvalidPack
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return ErrInvalidPack
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}
		if len(mandates) == maxRules {
			return ErrPackLimit
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return ErrInvalidPack
		}
		id := filepath.ToSlash(rel)
		if !validMandateID(id) {
			return ErrInvalidRule
		}
		info, err := entry.Info()
		if err != nil || info.Size() <= 0 || info.Size() > maxMarkdownFileBytes {
			return ErrPackLimit
		}
		if int64(total)+info.Size() > maxMarkdownPackBytes {
			return ErrPackLimit
		}
		input, err := confined.Open(id)
		if err != nil {
			return ErrInvalidPack
		}
		data, readErr := io.ReadAll(io.LimitReader(input, maxMarkdownFileBytes+1))
		closeErr := input.Close()
		if readErr != nil || closeErr != nil {
			return ErrInvalidPack
		}
		if len(data) == 0 || len(data) > maxMarkdownFileBytes || !utf8.Valid(data) || strings.ContainsRune(string(data), 0) || strings.TrimSpace(string(data)) == "" {
			return ErrInvalidRule
		}
		if _, _, err := parseMandate(string(data)); err != nil {
			return err
		}
		total += len(data)
		if total > maxMarkdownPackBytes {
			return ErrPackLimit
		}
		mandates = append(mandates, Mandate{ID: id, Body: string(data)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(mandates, func(i, j int) bool { return mandates[i].ID < mandates[j].ID })
	return mandates, nil
}

// CompileMandates applies the same fixed policy to built-in and loaded Markdown.
func CompileMandates(mandates []Mandate) (Pack, error) {
	if len(mandates) == 0 {
		return Pack{}, ErrInvalidPack
	}
	if len(mandates) > maxRules {
		return Pack{}, ErrPackLimit
	}
	ordered := append([]Mandate(nil), mandates...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	pack := Pack{rules: make([]Rule, 0, len(ordered))}
	total := 0
	for i, mandate := range ordered {
		if !validMandateID(mandate.ID) || !utf8.ValidString(mandate.Body) ||
			strings.ContainsRune(mandate.Body, 0) || strings.TrimSpace(mandate.Body) == "" ||
			len(mandate.Body) > maxMarkdownFileBytes || (i > 0 && ordered[i-1].ID == mandate.ID) {
			return Pack{}, ErrInvalidRule
		}
		total += len(mandate.Body)
		if total > maxMarkdownPackBytes {
			return Pack{}, ErrPackLimit
		}
		body, metadata, err := parseMandate(mandate.Body)
		if err != nil {
			return Pack{}, err
		}
		threshold, severity, title := 0.95, Medium, "Possible rule violation"
		if metadata.Threshold != nil {
			threshold = *metadata.Threshold
		}
		if metadata.Severity != nil {
			severity = *metadata.Severity
		}
		if metadata.Title != nil {
			title = *metadata.Title
		}
		message := strings.TrimSpace(body)
		if message == "" {
			message = "Changed code may violate " + mandate.ID + "."
		}
		instructions := "Does the code changed directly within this span violate this requirement? Do not flag code that merely invokes or dispatches to other functions unless the violation occurs directly in these changed lines.\n\n" + body
		pack.rules = append(pack.rules, Rule{ID: mandate.ID, Type: "noul", Body: body,
			Instructions: instructions,
			Threshold:    threshold, Severity: severity, Title: title,
			Message: message})
	}
	return pack, nil
}

func parseMandate(input string) (string, mandateMetadata, error) {
	if !strings.HasPrefix(input, "---\n") && !strings.HasPrefix(input, "---\r\n") {
		if strings.TrimSpace(input) == "" {
			return "", mandateMetadata{}, ErrInvalidRule
		}
		return input, mandateMetadata{}, nil
	}
	start := strings.IndexByte(input, '\n') + 1
	for cursor := start; cursor < len(input); {
		lineEnd := strings.IndexByte(input[cursor:], '\n')
		if lineEnd < 0 {
			lineEnd = len(input)
		} else {
			lineEnd += cursor
		}
		line := strings.TrimSuffix(input[cursor:lineEnd], "\r")
		if line == "---" {
			bodyStart := lineEnd
			if bodyStart < len(input) {
				bodyStart++
			}
			body := input[bodyStart:]
			if strings.TrimSpace(body) == "" || cursor-start > 1024 {
				return "", mandateMetadata{}, ErrInvalidRule
			}
			var document yaml.Node
			decoder := yaml.NewDecoder(bytes.NewBufferString(input[start:cursor]))
			if err := decoder.Decode(&document); err != nil || len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode ||
				validateNode(&document) != nil {
				return "", mandateMetadata{}, ErrInvalidRule
			}
			var extra yaml.Node
			if err := decoder.Decode(&extra); err != io.EOF {
				return "", mandateMetadata{}, ErrInvalidRule
			}
			fields := document.Content[0].Content
			for i := 0; i < len(fields); i += 2 {
				key, value := fields[i], fields[i+1]
				switch key.Value {
				case "severity", "title":
					if value.Tag != "!!str" {
						return "", mandateMetadata{}, ErrInvalidRule
					}
				case "threshold":
					if value.Tag != "!!float" && value.Tag != "!!int" {
						return "", mandateMetadata{}, ErrInvalidRule
					}
				}
			}
			var metadata mandateMetadata
			decoder = yaml.NewDecoder(bytes.NewBufferString(input[start:cursor]))
			decoder.KnownFields(true)
			if err := decoder.Decode(&metadata); err != nil || !validMandateMetadata(metadata) {
				return "", mandateMetadata{}, ErrInvalidRule
			}
			return body, metadata, nil
		}
		if lineEnd == len(input) || lineEnd-start > 1024 {
			break
		}
		cursor = lineEnd + 1
	}
	return "", mandateMetadata{}, ErrInvalidRule
}

func validMandateMetadata(metadata mandateMetadata) bool {
	if metadata.Severity != nil {
		switch *metadata.Severity {
		case Low, Medium, High, Critical:
		default:
			return false
		}
	}
	if metadata.Threshold != nil && (math.IsNaN(*metadata.Threshold) || math.IsInf(*metadata.Threshold, 0) || *metadata.Threshold < 0 || *metadata.Threshold > 1) {
		return false
	}
	return metadata.Title == nil || boundedText(*metadata.Title, maxTitle) && !strings.ContainsAny(*metadata.Title, "\r\n")
}

// WithPolicy applies one trusted run policy to every mandate in a pack.
func WithPolicy(pack Pack, threshold float64, severity Severity) (Pack, error) {
	return WithOverrides(pack, &threshold, &severity)
}

// WithOverrides applies only explicitly provided run values, preserving rule metadata otherwise.
func WithOverrides(pack Pack, threshold *float64, severity *Severity) (Pack, error) {
	if threshold != nil && (math.IsNaN(*threshold) || math.IsInf(*threshold, 0) || *threshold < 0 || *threshold > 1) ||
		severity != nil && (*severity != Low && *severity != Medium && *severity != High && *severity != Critical) || len(pack.rules) == 0 {
		return Pack{}, ErrInvalidRule
	}
	out := Pack{rules: pack.Rules()}
	for i := range out.rules {
		if threshold != nil {
			out.rules[i].Threshold = *threshold
		}
		if severity != nil {
			out.rules[i].Severity = *severity
		}
	}
	return out, nil
}

func validMandateID(id string) bool {
	if len(id) == 0 || len(id) > maxMarkdownIDBytes || !fs.ValidPath(id) || !strings.HasSuffix(id, ".md") || path.Base(id) == ".md" {
		return false
	}
	for _, r := range id {
		if r < ' ' || r == 0x7f || r == '\\' {
			return false
		}
	}
	return true
}
