package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultBlobBytes = 2 << 20
const defaultTotalBlobBytes = 16 << 20
const defaultMaxItems = 10_000
const hardMaxPatchBytes = 64 << 20
const hardMaxBlobBytes = 16 << 20
const hardMaxTotalBlobBytes = 128 << 20
const hardMaxItems = 100_000

var ErrInvalidLimits = errors.New("invalid Git input limits")
var ErrMissingSource = errors.New("work item source is unavailable")

// Limits cap subprocess output, each source blob, and the number of work items.
// Zero fields select finite defaults.
type Limits struct {
	MaxPatchBytes     int
	MaxBlobBytes      int
	MaxTotalBlobBytes int
	MaxItems          int
}

func (l Limits) normalized() (Limits, error) {
	if l.MaxPatchBytes < 0 || l.MaxBlobBytes < 0 || l.MaxTotalBlobBytes < 0 || l.MaxItems < 0 {
		return Limits{}, ErrInvalidLimits
	}
	if l.MaxPatchBytes == 0 {
		l.MaxPatchBytes = defaultOutputBytes
	}
	if l.MaxBlobBytes == 0 {
		l.MaxBlobBytes = defaultBlobBytes
	}
	if l.MaxTotalBlobBytes == 0 {
		l.MaxTotalBlobBytes = defaultTotalBlobBytes
	}
	if l.MaxItems == 0 {
		l.MaxItems = defaultMaxItems
	}
	if l.MaxPatchBytes > hardMaxPatchBytes || l.MaxBlobBytes > hardMaxBlobBytes ||
		l.MaxTotalBlobBytes > hardMaxTotalBlobBytes || l.MaxItems > hardMaxItems {
		return Limits{}, ErrInvalidLimits
	}
	return l, nil
}

// Repository reads only committed Git objects.
type Repository struct {
	runner commandRunner
	limits Limits
}

func NewRepository(dir string, limits Limits) (*Repository, error) {
	if dir == "" {
		return nil, errors.New("repository directory is required")
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	limits, err = limits.normalized()
	if err != nil {
		return nil, err
	}
	return &Repository{runner: commandRunner{dir: absolute}, limits: limits}, nil
}

// WorkItem anchors one contiguous changed span in a committed file side.
type WorkItem struct {
	ID        string
	OldPath   string
	NewPath   string
	Path      string
	Side      Side
	StartLine int
	EndLine   int
	Hunk      int
	sourceKey string
}

type SkipReason string

const (
	SkipBinary     SkipReason = "binary"
	SkipNonRegular SkipReason = "non_regular"
	SkipNoLines    SkipReason = "no_changed_lines"
)

type Skip struct {
	OldPath string
	NewPath string
	Reason  SkipReason
}

// Result holds a complete comparison. Sources are copied out on request.
type Result struct {
	Revisions Revisions
	Items     []WorkItem
	Skips     []Skip
	sources   map[string][]byte
}

// Source returns a copy of the committed file side used by an item.
func (r Result) Source(item WorkItem) ([]byte, error) {
	if item.sourceKey == "" {
		return nil, ErrMissingSource
	}
	source, ok := r.sources[item.sourceKey]
	if !ok {
		return nil, ErrMissingSource
	}
	return append([]byte(nil), source...), nil
}

// Compare resolves an immutable committed range and returns no partial result
// on command, parser, or resource-limit failure.
func (r *Repository) Compare(ctx context.Context, base, head string) (Result, error) {
	revs, err := r.runner.resolve(ctx, base, head)
	if err != nil {
		return Result{}, err
	}
	flags := []string{"--no-ext-diff", "--no-textconv", "--no-color", "--find-renames", "--no-relative", "--src-prefix=a/", "--dst-prefix=b/", "--diff-algorithm=myers", "--no-indent-heuristic", "--submodule=short"}
	rawArgs := append([]string{"diff", "--raw", "-z"}, flags...)
	rawArgs = append(rawArgs, revs.MergeBase, revs.Head, "--")
	raw, err := r.runner.run(ctx, r.limits.MaxPatchBytes, rawArgs...)
	if err != nil {
		return Result{}, fmt.Errorf("diff metadata: %w", err)
	}
	files, err := parseRaw(raw)
	if err != nil {
		return Result{}, err
	}
	patchArgs := append([]string{"diff", "--patch", "--unified=0"}, flags...)
	patchArgs = append(patchArgs, revs.MergeBase, revs.Head, "--")
	patch, err := r.runner.run(ctx, r.limits.MaxPatchBytes, patchArgs...)
	if err != nil {
		return Result{}, fmt.Errorf("diff patch: %w", err)
	}
	changes, err := parsePatch(patch, files)
	if err != nil {
		return Result{}, err
	}
	result := Result{Revisions: revs, sources: make(map[string][]byte)}
	totalSourceBytes := 0
	for _, change := range changes {
		if change.binary {
			result.Skips = append(result.Skips, Skip{OldPath: change.file.oldPath, NewPath: change.file.newPath, Reason: SkipBinary})
			continue
		}
		if !regularChange(change.file) {
			result.Skips = append(result.Skips, Skip{OldPath: change.file.oldPath, NewPath: change.file.newPath, Reason: SkipNonRegular})
			continue
		}
		if len(change.spans) == 0 {
			result.Skips = append(result.Skips, Skip{OldPath: change.file.oldPath, NewPath: change.file.newPath, Reason: SkipNoLines})
			continue
		}
		for _, span := range change.spans {
			if len(result.Items) >= r.limits.MaxItems {
				return Result{}, ErrLimit
			}
			item := WorkItem{OldPath: change.file.oldPath, NewPath: change.file.newPath, Side: span.side, StartLine: span.start, EndLine: span.end, Hunk: span.hunk}
			objectID := revs.Head
			item.Path = change.file.newPath
			if span.side == Left {
				objectID = revs.MergeBase
				item.Path = change.file.oldPath
			}
			if item.Path == "" || item.StartLine < 1 || item.EndLine < item.StartLine {
				return Result{}, ErrMalformedDiff
			}
			item.sourceKey = objectID + ":" + item.Path
			if _, ok := result.sources[item.sourceKey]; !ok {
				source, err := r.readBlob(ctx, item.sourceKey)
				if err != nil {
					return Result{}, err
				}
				if len(source) > r.limits.MaxTotalBlobBytes-totalSourceBytes {
					return Result{}, ErrLimit
				}
				totalSourceBytes += len(source)
				result.sources[item.sourceKey] = source
			}
			if item.EndLine > sourceLineCount(result.sources[item.sourceKey]) {
				return Result{}, ErrMalformedDiff
			}
			item.ID = itemID(revs, item)
			result.Items = append(result.Items, item)
		}
	}
	return result, nil
}

func sourceLineCount(source []byte) int {
	if len(source) == 0 {
		return 0
	}
	count := bytes.Count(source, []byte{'\n'})
	if source[len(source)-1] != '\n' {
		count++
	}
	return count
}

func regularChange(f rawFile) bool {
	for _, mode := range []string{f.oldMode, f.newMode} {
		if mode != "000000" && mode != "100644" && mode != "100755" {
			return false
		}
	}
	return true
}

func (r *Repository) readBlob(ctx context.Context, spec string) ([]byte, error) {
	sizeBytes, err := r.runner.run(ctx, 64, "cat-file", "-s", spec)
	if err != nil {
		return nil, fmt.Errorf("source size: %w", err)
	}
	size, err := strconv.ParseInt(strings.TrimSpace(string(sizeBytes)), 10, 64)
	if err != nil || size < 0 {
		return nil, ErrMalformedDiff
	}
	if size > int64(r.limits.MaxBlobBytes) {
		return nil, ErrLimit
	}
	data, err := r.runner.run(ctx, r.limits.MaxBlobBytes, "cat-file", "blob", spec)
	if err != nil {
		return nil, fmt.Errorf("source blob: %w", err)
	}
	if int64(len(data)) != size {
		return nil, ErrMalformedDiff
	}
	return data, nil
}

func itemID(revs Revisions, item WorkItem) string {
	h := sha256.New()
	for _, part := range []string{revs.MergeBase, revs.Head, item.OldPath, item.NewPath, string(item.Side),
		strconv.Itoa(item.StartLine), strconv.Itoa(item.EndLine), strconv.Itoa(item.Hunk)} {
		writeIDPart(h, part)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeIDPart(h hash.Hash, part string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(part)))
	_, _ = h.Write(length[:])
	_, _ = h.Write([]byte(part))
}
