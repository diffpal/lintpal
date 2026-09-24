package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

var ErrExport = errors.New("lint report export failed")
var ErrGate = errors.New("lint severity gate failed")
var ErrInvalidFormat = errors.New("invalid lint report format")
var ErrInvalidThreshold = errors.New("invalid lint severity threshold")

type Format string

const (
	JSON  Format = "json"
	Human Format = "human"
)

type Threshold string

const (
	None     Threshold = "none"
	Low      Threshold = "low"
	Medium   Threshold = "medium"
	High     Threshold = "high"
	Critical Threshold = "critical"
)

func WriteJSON(writer io.Writer, artifact Report) error {
	if err := Validate(artifact); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return ErrInvalidReport
	}
	return writeComplete(writer, append(payload, '\n'))
}

func WriteHuman(writer io.Writer, artifact Report) error {
	if err := Validate(artifact); err != nil {
		return err
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "lintpal %s base=%s head=%s merge_base=%s\n", artifact.SchemaVersion, artifact.BaseSHA, artifact.HeadSHA, artifact.MergeBaseSHA)
	for _, d := range artifact.Diagnostics {
		fmt.Fprintf(&b, "%s %s %s:%d-%d rule=%s title=%s message=%s evidence=%s:%.6g",
			d.Severity, d.Side, strconv.Quote(d.Path), d.StartLine, d.EndLine,
			strconv.Quote(d.RuleID), strconv.Quote(d.Title), strconv.Quote(d.Message),
			d.Evidence.Kind, d.Evidence.Value)
		if d.Evidence.Confidence != nil {
			fmt.Fprintf(&b, " confidence=%.6g", *d.Evidence.Confidence)
		}
		fmt.Fprintf(&b, " provider=%s model=%s\n", strconv.Quote(d.Provider), strconv.Quote(d.Model))
	}
	for _, skip := range artifact.Skips {
		fmt.Fprintf(&b, "skip reason=%s old=%s new=%s\n", skip.Reason, strconv.Quote(skip.OldPath), strconv.Quote(skip.NewPath))
	}
	fmt.Fprintf(&b, "stats work_items=%d skipped=%d groups=%d batches=%d questions=%d diagnostics=%d input_tokens=%d output_tokens=%d\n",
		artifact.Stats.WorkItems, artifact.Stats.Skipped, artifact.Stats.Groups, artifact.Stats.Batches,
		artifact.Stats.Questions, artifact.Stats.Diagnostics, artifact.Stats.InputTokens, artifact.Stats.OutputTokens)
	return writeComplete(writer, b.Bytes())
}

func WriteAndGate(writer io.Writer, artifact Report, format Format, threshold Threshold) error {
	if _, err := rank(threshold); err != nil {
		return err
	}
	artifact = WithGate(artifact, threshold)
	var err error
	switch format {
	case JSON:
		err = WriteJSON(writer, artifact)
	case Human:
		err = WriteHuman(writer, artifact)
	default:
		return ErrInvalidFormat
	}
	if err != nil {
		return err
	}
	gate, err := MeetsGate(artifact, threshold)
	if err != nil {
		return err
	}
	if gate {
		return ErrGate
	}
	return nil
}

// WithGate binds the report's blocking flags to the selected CLI gate.
func WithGate(artifact Report, threshold Threshold) Report {
	artifact.gate = threshold
	return artifact
}

func MeetsGate(artifact Report, threshold Threshold) (bool, error) {
	if err := Validate(artifact); err != nil {
		return false, err
	}
	minimum, err := rank(threshold)
	if err != nil {
		return false, err
	}
	if threshold == None {
		return false, nil
	}
	for _, diagnostic := range artifact.Diagnostics {
		level, _ := rank(Threshold(diagnostic.Severity))
		if level >= minimum {
			return true, nil
		}
	}
	return false, nil
}

func rank(threshold Threshold) (int, error) {
	switch threshold {
	case None:
		return 5, nil
	case Low:
		return 1, nil
	case Medium:
		return 2, nil
	case High:
		return 3, nil
	case Critical:
		return 4, nil
	default:
		return 0, ErrInvalidThreshold
	}
}

func writeComplete(writer io.Writer, payload []byte) error {
	if writer == nil {
		return ErrExport
	}
	n, err := writer.Write(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrExport, err)
	}
	if n != len(payload) {
		return fmt.Errorf("%w: %v", ErrExport, io.ErrShortWrite)
	}
	return nil
}
