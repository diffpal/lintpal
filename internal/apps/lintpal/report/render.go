package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var ErrExport = errors.New("lint report export failed")
var ErrGate = errors.New("lint severity gate failed")
var ErrInvalidFormat = errors.New("invalid lint report format")
var ErrInvalidThreshold = errors.New("invalid lint severity threshold")

type Format string

const (
	JSON     Format = "json"
	Markdown Format = "markdown"
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

func WriteAndGate(writer io.Writer, artifact Report, format Format, threshold Threshold) error {
	if _, err := rank(threshold); err != nil {
		return err
	}
	artifact = WithGate(artifact, threshold)
	var err error
	switch format {
	case JSON:
		err = WriteJSON(writer, artifact)
	case Markdown:
		err = WriteMarkdown(writer, artifact)
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
