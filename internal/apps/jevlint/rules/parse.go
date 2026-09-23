// Package rules loads declarative lint rules and maps typed Jev answers to
// deterministic decisions. Repository-provided rule files are untrusted data.
package rules

import (
	"bytes"
	"context"
	"errors"
	"io"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

var ErrInvalidPack = errors.New("invalid rule pack")
var ErrPackLimit = errors.New("rule pack limit exceeded")
var ErrUnsupportedSchema = errors.New("unsupported rule pack schema")

const maxFileBytes = 256 << 10
const schemaV1 = "jevlint.rules.v1"

type rawPack struct {
	Schema string    `yaml:"schema"`
	Rules  []rawRule `yaml:"rules"`
}

type rawRule struct {
	ID             string    `yaml:"id"`
	Type           string    `yaml:"type"`
	Instructions   string    `yaml:"instructions"`
	Criteria       yaml.Node `yaml:"criteria"`
	TriggerChoices []string  `yaml:"trigger_choices"`
	Threshold      *float64  `yaml:"threshold"`
	Severity       string    `yaml:"severity"`
	Title          string    `yaml:"title"`
	Message        string    `yaml:"message"`
	Paths          []string  `yaml:"paths"`
	Sides          []string  `yaml:"sides"`
}

// parse checks the YAML syntax and shape before semantic rule validation.
// It never returns source text or a YAML parser error to the caller.
func parse(reader io.Reader) (rawPack, error) {
	return parseContext(context.Background(), reader)
}

type cancelReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r cancelReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func parseContext(ctx context.Context, reader io.Reader) (rawPack, error) {
	if err := ctx.Err(); err != nil {
		return rawPack{}, err
	}
	if reader == nil {
		return rawPack{}, ErrInvalidPack
	}
	data, err := io.ReadAll(io.LimitReader(cancelReader{ctx, reader}, maxFileBytes+1))
	if err != nil {
		if ctx.Err() != nil {
			return rawPack{}, ctx.Err()
		}
		return rawPack{}, ErrInvalidPack
	}
	if err := ctx.Err(); err != nil {
		return rawPack{}, err
	}
	if len(data) > maxFileBytes {
		return rawPack{}, ErrPackLimit
	}
	if len(data) == 0 || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return rawPack{}, ErrInvalidPack
	}
	var document yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&document); err != nil || len(document.Content) != 1 {
		return rawPack{}, ErrInvalidPack
	}
	if err := validateNode(&document); err != nil {
		return rawPack{}, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		return rawPack{}, ErrInvalidPack
	}
	decoder = yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var pack rawPack
	if err := decoder.Decode(&pack); err != nil {
		return rawPack{}, ErrInvalidPack
	}
	if pack.Schema != schemaV1 {
		return rawPack{}, ErrUnsupportedSchema
	}
	return pack, nil
}

func validateNode(node *yaml.Node) error {
	if node == nil || node.Alias != nil || node.Anchor != "" {
		return ErrInvalidPack
	}
	switch node.Tag {
	case "", "!!map", "!!seq", "!!str", "!!int", "!!float", "!!bool", "!!null":
	default:
		return ErrInvalidPack
	}
	if node.Kind == yaml.AliasNode {
		return ErrInvalidPack
	}
	if node.Kind == yaml.MappingNode {
		if len(node.Content)%2 != 0 {
			return ErrInvalidPack
		}
		seen := make(map[string]bool, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || key.Value == "<<" || seen[key.Value] {
				return ErrInvalidPack
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := validateNode(child); err != nil {
			return err
		}
	}
	return nil
}
