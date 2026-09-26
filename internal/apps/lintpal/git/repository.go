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
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	diffutil "github.com/go-git/go-git/v5/utils/diff"
)

const defaultBlobBytes = 2 << 20
const defaultTotalBlobBytes = 16 << 20
const defaultMaxItems = 10_000
const hardMaxPatchBytes = 64 << 20
const hardMaxBlobBytes = 16 << 20
const hardMaxTotalBlobBytes = 128 << 20
const hardMaxItems = 100_000
const defaultOutputBytes = 8 << 20

var ErrInvalidLimits = errors.New("invalid Git input limits")
var ErrMissingSource = errors.New("work item source is unavailable")
var ErrLimit = errors.New("git input limit exceeded")
var ErrInvalidRevision = errors.New("invalid commit revision")
var ErrAmbiguousBase = errors.New("comparison has no unique merge base")

// Revisions contains only verified immutable commit IDs.
type Revisions struct {
	Base      string
	Head      string
	MergeBase string
}

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

// Repository reads Git objects and working tree state using pure Go.
type Repository struct {
	client *gitClient
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
	client, err := newGitClient(absolute)
	if err != nil {
		return nil, err
	}
	return &Repository{client: client, limits: limits}, nil
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
	revs, err := r.client.resolve(ctx, base, head)
	if err != nil {
		return Result{}, err
	}
	baseCommit, err := r.client.resolveCommit(ctx, revs.MergeBase)
	if err != nil {
		return Result{}, fmt.Errorf("merge base commit: %w", err)
	}
	headCommit, err := r.client.resolveCommit(ctx, revs.Head)
	if err != nil {
		return Result{}, fmt.Errorf("head commit: %w", err)
	}
	changes, err := r.client.diffCommits(ctx, baseCommit, headCommit, r.limits.MaxPatchBytes)
	if err != nil {
		return Result{}, err
	}
	return r.buildResult(ctx, revs, changes, false)
}

// CompareUncommitted inspects unstaged, staged, and untracked changes in the working tree.
func (r *Repository) CompareUncommitted(ctx context.Context) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	headID := "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	headCommit, _ := r.client.resolveCommit(ctx, "HEAD")
	if headCommit != nil {
		headID = headCommit.Hash.String()
	}

	w, err := r.client.repo.Worktree()
	if err != nil {
		return Result{}, err
	}
	status, err := w.Status()
	if err != nil {
		return Result{}, err
	}

	headFiles := make(map[string]rawTreeEntry)
	if headCommit != nil {
		_ = r.client.flattenRawTree(headCommit.TreeHash, "", headFiles)
	}

	paths := make([]string, 0, len(status))
	for p := range status {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var changes []fileChange
	var skips []Skip

	for _, p := range paths {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		clean := filepath.Clean(p)
		if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			continue
		}
		fs := status[p]
		if fs.Staging == git.Unmodified && fs.Worktree == git.Unmodified {
			continue
		}

		headEntry, inHead := headFiles[clean]

		if fs.Worktree == git.Deleted || fs.Staging == git.Deleted {
			if inHead {
				headContent, err := r.client.readBlobByHash(headEntry.Hash)
				if err != nil {
					return Result{}, err
				}
				if isBinary(headContent) {
					changes = append(changes, fileChange{
						file:   rawFile{status: 'D', oldMode: normalizeMode(headEntry.Mode), newMode: "000000", oldPath: clean},
						binary: true,
					})
				} else {
					lines := sourceLineCount(headContent)
					changes = append(changes, fileChange{
						file: rawFile{status: 'D', oldMode: normalizeMode(headEntry.Mode), newMode: "000000", oldPath: clean},
						spans: []changedSpan{{
							side:  Left,
							start: 1,
							end:   lines,
							hunk:  1,
						}},
					})
				}
			}
			continue
		}

		// File is present in working tree
		full := filepath.Join(r.client.dir, clean)
		info, err := os.Lstat(full)
		if err != nil {
			continue
		}

		if fs.Worktree == git.Untracked {
			if !info.Mode().IsRegular() {
				skips = append(skips, Skip{NewPath: clean, Reason: SkipNonRegular})
				continue
			}
			if info.Size() > int64(r.limits.MaxBlobBytes) {
				return Result{}, ErrLimit
			}
			content, err := os.ReadFile(full)
			if err != nil {
				return Result{}, fmt.Errorf("read untracked %q: %w", clean, err)
			}
			if isBinary(content) {
				skips = append(skips, Skip{NewPath: clean, Reason: SkipBinary})
				continue
			}
			lines := sourceLineCount(content)
			if lines == 0 {
				skips = append(skips, Skip{NewPath: clean, Reason: SkipNoLines})
				continue
			}
			changes = append(changes, fileChange{
				file: rawFile{
					oldMode: "000000",
					newMode: "100644",
					status:  'A',
					newPath: clean,
				},
				spans: []changedSpan{{
					side:  Right,
					start: 1,
					end:   lines,
					hunk:  1,
				}},
			})
			continue
		}

		if !inHead {
			// Staged new file
			if !info.Mode().IsRegular() {
				skips = append(skips, Skip{NewPath: clean, Reason: SkipNonRegular})
				continue
			}
			if info.Size() > int64(r.limits.MaxBlobBytes) {
				return Result{}, ErrLimit
			}
			content, err := r.readFile(clean)
			if err != nil {
				return Result{}, err
			}
			if isBinary(content) {
				changes = append(changes, fileChange{
					file:   rawFile{status: 'A', oldMode: "000000", newMode: "100644", newPath: clean},
					binary: true,
				})
			} else {
				lines := sourceLineCount(content)
				changes = append(changes, fileChange{
					file: rawFile{status: 'A', oldMode: "000000", newMode: "100644", newPath: clean},
					spans: []changedSpan{{
						side:  Right,
						start: 1,
						end:   lines,
						hunk:  1,
					}},
				})
			}
			continue
		}

		// Modified file (both in HEAD and on disk)
		if !info.Mode().IsRegular() {
			skips = append(skips, Skip{OldPath: clean, NewPath: clean, Reason: SkipNonRegular})
			continue
		}
		headContent, err := r.client.readBlobByHash(headEntry.Hash)
		if err != nil {
			return Result{}, err
		}
		content, err := r.readFile(clean)
		if err != nil {
			return Result{}, err
		}
		if isBinary(headContent) || isBinary(content) {
			changes = append(changes, fileChange{
				file:   rawFile{status: 'M', oldMode: normalizeMode(headEntry.Mode), newMode: normalizeMode(headEntry.Mode), oldPath: clean, newPath: clean},
				binary: true,
			})
		} else {
			diffs := diffutil.Do(string(headContent), string(content))
			spans := spansFromDMP(diffs)
			changes = append(changes, fileChange{
				file:  rawFile{status: 'M', oldMode: normalizeMode(headEntry.Mode), newMode: normalizeMode(headEntry.Mode), oldPath: clean, newPath: clean},
				spans: spans,
			})
		}
	}

	revs := Revisions{Base: headID, Head: "UNCOMMITTED", MergeBase: headID}
	result, err := r.buildResult(ctx, revs, changes, true)
	if err != nil {
		return Result{}, err
	}
	result.Skips = append(result.Skips, skips...)
	return result, nil
}

func (r *Repository) buildResult(ctx context.Context, revs Revisions, changes []fileChange, uncommitted bool) (Result, error) {
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
			if uncommitted && span.side == Right {
				item.sourceKey = "WORKTREE:" + item.Path
				if _, ok := result.sources[item.sourceKey]; !ok {
					source, err := r.readFile(item.Path)
					if err != nil {
						return Result{}, err
					}
					if len(source) > r.limits.MaxTotalBlobBytes-totalSourceBytes {
						return Result{}, ErrLimit
					}
					totalSourceBytes += len(source)
					result.sources[item.sourceKey] = source
				}
			} else {
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

func (r *Repository) readFile(relPath string) ([]byte, error) {
	clean := filepath.Clean(relPath)
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return nil, ErrMalformedDiff
	}
	full := filepath.Join(r.client.dir, clean)
	info, err := os.Lstat(full)
	if err != nil {
		return nil, fmt.Errorf("read worktree file %q: %w", relPath, err)
	}
	if !info.Mode().IsRegular() {
		return nil, ErrMalformedDiff
	}
	if info.Size() > int64(r.limits.MaxBlobBytes) {
		return nil, ErrLimit
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, fmt.Errorf("read worktree file %q: %w", relPath, err)
	}
	return data, nil
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
	commitSHA, relPath, ok := strings.Cut(spec, ":")
	if !ok {
		return nil, ErrMalformedDiff
	}
	return r.client.readBlob(ctx, commitSHA, relPath, r.limits.MaxBlobBytes)
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
