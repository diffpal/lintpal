package git

import (
	"bytes"
	"strconv"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	diff "github.com/go-git/go-git/v5/plumbing/format/diff"
	diffutil "github.com/go-git/go-git/v5/utils/diff"
)

// patchBudget validates encoded unified patches without storing another copy.
// A single budget is shared by every changed file in a comparison.
type patchBudget struct{ remaining, lineBytes int }

func (b *patchBudget) Write(data []byte) (int, error) {
	if len(data) > b.remaining {
		return 0, ErrLimit
	}
	for rest := data; len(rest) > 0; {
		i := bytes.IndexByte(rest, '\n')
		if i < 0 {
			b.lineBytes += len(rest)
			rest = nil
		} else {
			b.lineBytes += i
			if b.lineBytes > maxDiffLineBytes {
				return 0, ErrLimit
			}
			b.lineBytes = 0
			rest = rest[i+1:]
		}
		if b.lineBytes > maxDiffLineBytes {
			return 0, ErrLimit
		}
	}
	b.remaining -= len(data)
	return len(data), nil
}

func (b *patchBudget) add(patch diff.Patch) error {
	for _, fp := range patch.FilePatches() {
		from, to := fp.Files()
		for _, file := range []diff.File{from, to} {
			if file != nil && len(file.Path()) > maxGitPathBytes {
				return ErrLimit
			}
		}
	}
	return diff.NewUnifiedEncoder(b, diff.DefaultContextLines).Encode(patch)
}

func (b *patchBudget) addRaw(file rawFile, before, after []byte) ([]changedSpan, error) {
	fp := rawFilePatch{file: file, binary: isBinary(before) || isBinary(after), beforeHash: plumbing.ComputeHash(plumbing.BlobObject, before), afterHash: plumbing.ComputeHash(plumbing.BlobObject, after)}
	var spans []changedSpan
	if !fp.binary {
		changes := diffutil.Do(string(before), string(after))
		spans = spansFromDMP(changes)
		for _, change := range changes {
			op := diff.Equal
			if change.Type < 0 {
				op = diff.Delete
			} else if change.Type > 0 {
				op = diff.Add
			}
			fp.chunks = append(fp.chunks, rawChunk{content: change.Text, op: op})
		}
	}
	if err := b.add(rawPatch{fp}); err != nil {
		return nil, err
	}
	return spans, nil
}

type rawPatch []diff.FilePatch

func (p rawPatch) FilePatches() []diff.FilePatch { return p }
func (rawPatch) Message() string                 { return "" }

type rawFilePatch struct {
	file                  rawFile
	binary                bool
	chunks                []diff.Chunk
	beforeHash, afterHash plumbing.Hash
}

func (p rawFilePatch) IsBinary() bool       { return p.binary }
func (p rawFilePatch) Chunks() []diff.Chunk { return p.chunks }
func (p rawFilePatch) Files() (diff.File, diff.File) {
	var from, to diff.File
	if p.file.oldPath != "" {
		from = patchFile{path: p.file.oldPath, mode: p.file.oldMode, hash: p.beforeHash}
	}
	if p.file.newPath != "" {
		to = patchFile{path: p.file.newPath, mode: p.file.newMode, hash: p.afterHash}
	}
	return from, to
}

type patchFile struct {
	path, mode string
	hash       plumbing.Hash
}

func (f patchFile) Path() string        { return f.path }
func (f patchFile) Hash() plumbing.Hash { return f.hash }
func (f patchFile) Mode() filemode.FileMode {
	mode, _ := strconv.ParseUint(f.mode, 8, 32)
	return filemode.FileMode(mode)
}

type rawChunk struct {
	content string
	op      diff.Operation
}

func (c rawChunk) Content() string      { return c.content }
func (c rawChunk) Type() diff.Operation { return c.op }
