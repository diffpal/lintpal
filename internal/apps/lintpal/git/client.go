package git

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	diff "github.com/go-git/go-git/v5/plumbing/format/diff"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage"
	diffutil "github.com/go-git/go-git/v5/utils/diff"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type gitClient struct {
	dir  string
	repo *git.Repository
}

func newGitClient(dir string) (*gitClient, error) {
	repo, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return nil, fmt.Errorf("open git repository: %w", err)
	}
	return &gitClient{dir: dir, repo: repo}, nil
}

func (c *gitClient) resolve(ctx context.Context, base, head string) (Revisions, error) {
	if err := ctx.Err(); err != nil {
		return Revisions{}, err
	}
	baseCommit, err := c.resolveCommit(ctx, base)
	if err != nil {
		return Revisions{}, fmt.Errorf("base: %w", err)
	}
	headCommit, err := c.resolveCommit(ctx, head)
	if err != nil {
		return Revisions{}, fmt.Errorf("head: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Revisions{}, err
	}
	bases, err := baseCommit.MergeBase(headCommit)
	if err != nil {
		return Revisions{}, fmt.Errorf("merge base: %w", ErrAmbiguousBase)
	}
	if len(bases) != 1 {
		return Revisions{}, ErrAmbiguousBase
	}
	return Revisions{
		Base:      baseCommit.Hash.String(),
		Head:      headCommit.Hash.String(),
		MergeBase: bases[0].Hash.String(),
	}, nil
}

func (c *gitClient) resolveCommit(ctx context.Context, rev string) (*object.Commit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cleanRev := strings.TrimSpace(rev)
	if cleanRev == "" || strings.HasPrefix(cleanRev, "-") || strings.ContainsAny(cleanRev, "\x00\r\n\t") {
		return nil, ErrInvalidRevision
	}
	cleanRev = strings.TrimSuffix(cleanRev, "^{commit}")

	// Direct hash lookup if 40-character hex
	if validObjectID(cleanRev) {
		hash := plumbing.NewHash(cleanRev)
		if commit, err := c.repo.CommitObject(hash); err == nil {
			return commit, nil
		}
	}

	// Resolve via go-git revision syntax
	hash, err := c.repo.ResolveRevision(plumbing.Revision(cleanRev))
	if err != nil {
		hash, err = c.resolvePrefixOrRef(cleanRev)
		if err != nil {
			return nil, ErrInvalidRevision
		}
	}

	commit, err := c.repo.CommitObject(*hash)
	if err != nil {
		if tag, tagErr := c.repo.TagObject(*hash); tagErr == nil {
			if targetCommit, cErr := tag.Commit(); cErr == nil {
				return targetCommit, nil
			}
		}
		return nil, ErrInvalidRevision
	}
	return commit, nil
}

func (c *gitClient) resolvePrefixOrRef(rev string) (*plumbing.Hash, error) {
	for _, prefix := range []string{"refs/heads/", "refs/remotes/", "refs/tags/"} {
		ref, err := c.repo.Reference(plumbing.ReferenceName(prefix+rev), true)
		if err == nil {
			h := ref.Hash()
			return &h, nil
		}
	}
	return nil, ErrInvalidRevision
}

func (c *gitClient) diffCommits(ctx context.Context, baseCommit, headCommit *object.Commit, maxPatchBytes int) ([]fileChange, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	baseTree, err1 := baseCommit.Tree()
	headTree, err2 := headCommit.Tree()
	if err1 == nil && err2 == nil {
		changes, err := c.diffTrees(ctx, baseTree, headTree, maxPatchBytes)
		if err == nil {
			return changes, nil
		}
		if !strings.Contains(err.Error(), "contains control character") {
			return nil, err
		}
	}
	return c.diffCommitsRaw(ctx, baseCommit, headCommit, maxPatchBytes)
}

func (c *gitClient) diffTrees(ctx context.Context, baseTree, headTree *object.Tree, maxPatchBytes int) ([]fileChange, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	changes, err := object.DiffTreeWithOptions(ctx, baseTree, headTree, &object.DiffTreeOptions{
		DetectRenames: true,
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("diff metadata: %w", err)
	}
	patch, err := changes.PatchContext(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("diff patch: %w", err)
	}
	patchBytes := len(patch.String())
	if patchBytes > maxPatchBytes {
		return nil, ErrLimit
	}

	var fileChanges []fileChange
	for _, fp := range patch.FilePatches() {
		from, to := fp.Files()
		var rf rawFile
		switch {
		case from == nil && to != nil:
			rf = rawFile{
				status:  'A',
				oldMode: "000000",
				newMode: fmt.Sprintf("%06o", to.Mode()),
				newPath: to.Path(),
			}
		case from != nil && to == nil:
			rf = rawFile{
				status:  'D',
				oldMode: fmt.Sprintf("%06o", from.Mode()),
				newMode: "000000",
				oldPath: from.Path(),
			}
		case from != nil && to != nil:
			if from.Path() != to.Path() {
				rf = rawFile{
					status:  'R',
					oldMode: fmt.Sprintf("%06o", from.Mode()),
					newMode: fmt.Sprintf("%06o", to.Mode()),
					oldPath: from.Path(),
					newPath: to.Path(),
				}
			} else {
				rf = rawFile{
					status:  'M',
					oldMode: fmt.Sprintf("%06o", from.Mode()),
					newMode: fmt.Sprintf("%06o", to.Mode()),
					oldPath: from.Path(),
					newPath: to.Path(),
				}
			}
		default:
			continue
		}

		if fp.IsBinary() {
			fileChanges = append(fileChanges, fileChange{
				file:   rf,
				binary: true,
			})
			continue
		}

		spans := spansFromFilePatch(fp)
		fileChanges = append(fileChanges, fileChange{
			file:  rf,
			spans: spans,
		})
	}
	return fileChanges, nil
}

func spansFromFilePatch(fp diff.FilePatch) []changedSpan {
	var spans []changedSpan
	oldLine := 0
	newLine := 0
	hunkOrdinal := 0
	inHunk := false

	for _, chunk := range fp.Chunks() {
		content := chunk.Content()
		numLines := sourceLineCount([]byte(content))
		switch chunk.Type() {
		case 0: // diff.Equal
			oldLine += numLines
			newLine += numLines
			inHunk = false
		case 2: // diff.Delete (Left side)
			if !inHunk {
				hunkOrdinal++
				inHunk = true
			}
			for i := 1; i <= numLines; i++ {
				spans = appendChanged(spans, Left, oldLine+i, hunkOrdinal)
			}
			oldLine += numLines
		case 1: // diff.Add (Right side)
			if !inHunk {
				hunkOrdinal++
				inHunk = true
			}
			for i := 1; i <= numLines; i++ {
				spans = appendChanged(spans, Right, newLine+i, hunkOrdinal)
			}
			newLine += numLines
		}
	}
	return spans
}

type rawTreeEntry struct {
	Mode string
	Name string
	Hash plumbing.Hash
}

func readRawTree(storer storage.Storer, hash plumbing.Hash) ([]rawTreeEntry, error) {
	obj, err := storer.EncodedObject(plumbing.TreeObject, hash)
	if err != nil {
		return nil, err
	}
	r, err := obj.Reader()
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var entries []rawTreeEntry
	buf := data
	for len(buf) > 0 {
		spaceIdx := bytes.IndexByte(buf, ' ')
		if spaceIdx < 0 {
			return nil, fmt.Errorf("malformed tree")
		}
		mode := string(buf[:spaceIdx])
		buf = buf[spaceIdx+1:]
		nulIdx := bytes.IndexByte(buf, 0)
		if nulIdx < 0 {
			return nil, fmt.Errorf("malformed tree")
		}
		name := string(buf[:nulIdx])
		buf = buf[nulIdx+1:]
		if len(buf) < 20 {
			return nil, fmt.Errorf("malformed tree: truncated hash")
		}
		var h plumbing.Hash
		copy(h[:], buf[:20])
		buf = buf[20:]
		entries = append(entries, rawTreeEntry{Mode: mode, Name: name, Hash: h})
	}
	return entries, nil
}

func (c *gitClient) flattenRawTree(hash plumbing.Hash, prefix string, out map[string]rawTreeEntry) error {
	entries, err := readRawTree(c.repo.Storer, hash)
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := e.Name
		if prefix != "" {
			path = prefix + "/" + e.Name
		}
		if e.Mode == "40000" || e.Mode == "040000" {
			if err := c.flattenRawTree(e.Hash, path, out); err != nil {
				return err
			}
		} else {
			out[path] = rawTreeEntry{Mode: e.Mode, Name: path, Hash: e.Hash}
		}
	}
	return nil
}

func (c *gitClient) diffCommitsRaw(ctx context.Context, baseCommit, headCommit *object.Commit, maxPatchBytes int) ([]fileChange, error) {
	baseFiles := make(map[string]rawTreeEntry)
	headFiles := make(map[string]rawTreeEntry)
	if err := c.flattenRawTree(baseCommit.TreeHash, "", baseFiles); err != nil {
		return nil, err
	}
	if err := c.flattenRawTree(headCommit.TreeHash, "", headFiles); err != nil {
		return nil, err
	}

	allPaths := make(map[string]bool)
	for p := range baseFiles {
		allPaths[p] = true
	}
	for p := range headFiles {
		allPaths[p] = true
	}
	sortedPaths := make([]string, 0, len(allPaths))
	for p := range allPaths {
		sortedPaths = append(sortedPaths, p)
	}
	sort.Strings(sortedPaths)

	var fileChanges []fileChange
	for _, p := range sortedPaths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		baseEntry, inBase := baseFiles[p]
		headEntry, inHead := headFiles[p]
		if inBase && inHead && baseEntry.Hash == headEntry.Hash && baseEntry.Mode == headEntry.Mode {
			continue
		}

		var rf rawFile
		switch {
		case !inBase && inHead:
			rf = rawFile{status: 'A', oldMode: "000000", newMode: normalizeMode(headEntry.Mode), newPath: p}
		case inBase && !inHead:
			rf = rawFile{status: 'D', oldMode: normalizeMode(baseEntry.Mode), newMode: "000000", oldPath: p}
		case inBase && inHead:
			rf = rawFile{status: 'M', oldMode: normalizeMode(baseEntry.Mode), newMode: normalizeMode(headEntry.Mode), oldPath: p, newPath: p}
		}

		var baseContent, headContent []byte
		if inBase {
			b, err := c.readBlobByHash(baseEntry.Hash)
			if err != nil {
				return nil, err
			}
			baseContent = b
		}
		if inHead {
			b, err := c.readBlobByHash(headEntry.Hash)
			if err != nil {
				return nil, err
			}
			headContent = b
		}

		if isBinary(baseContent) || isBinary(headContent) {
			fileChanges = append(fileChanges, fileChange{file: rf, binary: true})
			continue
		}

		diffs := diffutil.Do(string(baseContent), string(headContent))
		spans := spansFromDMP(diffs)
		fileChanges = append(fileChanges, fileChange{file: rf, spans: spans})
	}
	return fileChanges, nil
}

func spansFromDMP(diffs []diffmatchpatch.Diff) []changedSpan {
	var spans []changedSpan
	oldLine := 0
	newLine := 0
	hunkOrdinal := 0
	inHunk := false

	for _, d := range diffs {
		numLines := sourceLineCount([]byte(d.Text))
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			oldLine += numLines
			newLine += numLines
			inHunk = false
		case diffmatchpatch.DiffDelete:
			if !inHunk {
				hunkOrdinal++
				inHunk = true
			}
			for i := 1; i <= numLines; i++ {
				spans = appendChanged(spans, Left, oldLine+i, hunkOrdinal)
			}
			oldLine += numLines
		case diffmatchpatch.DiffInsert:
			if !inHunk {
				hunkOrdinal++
				inHunk = true
			}
			for i := 1; i <= numLines; i++ {
				spans = appendChanged(spans, Right, newLine+i, hunkOrdinal)
			}
			newLine += numLines
		}
	}
	return spans
}

func normalizeMode(mode string) string {
	for len(mode) < 6 {
		mode = "0" + mode
	}
	return mode
}

func isBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0
}

func (c *gitClient) readBlobByHash(hash plumbing.Hash) ([]byte, error) {
	obj, err := c.repo.Storer.EncodedObject(plumbing.BlobObject, hash)
	if err != nil {
		return nil, fmt.Errorf("read blob %s: %w", hash, err)
	}
	r, err := obj.Reader()
	if err != nil {
		return nil, err
	}
	defer func() { _ = r.Close() }()
	return io.ReadAll(r)
}

func (c *gitClient) findBlob(rootTreeHash plumbing.Hash, relPath string) (plumbing.Hash, string, error) {
	parts := strings.Split(relPath, "/")
	currentTreeHash := rootTreeHash
	for i, part := range parts {
		entries, err := readRawTree(c.repo.Storer, currentTreeHash)
		if err != nil {
			return plumbing.ZeroHash, "", err
		}
		found := false
		for _, e := range entries {
			if e.Name == part {
				found = true
				if i == len(parts)-1 {
					return e.Hash, e.Mode, nil
				}
				currentTreeHash = e.Hash
				break
			}
		}
		if !found {
			return plumbing.ZeroHash, "", ErrMissingSource
		}
	}
	return plumbing.ZeroHash, "", ErrMissingSource
}

func (c *gitClient) readBlob(ctx context.Context, commitSHA, relPath string, maxBlobBytes int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	commit, err := c.resolveCommit(ctx, commitSHA)
	if err != nil {
		return nil, fmt.Errorf("source size: %w", err)
	}
	blobHash, _, err := c.findBlob(commit.TreeHash, relPath)
	if err != nil {
		return nil, fmt.Errorf("source size: %w", err)
	}
	obj, err := c.repo.Storer.EncodedObject(plumbing.BlobObject, blobHash)
	if err != nil {
		return nil, fmt.Errorf("source size: %w", err)
	}
	if obj.Size() > int64(maxBlobBytes) {
		return nil, ErrLimit
	}
	reader, err := obj.Reader()
	if err != nil {
		return nil, fmt.Errorf("source blob: %w", err)
	}
	defer func() { _ = reader.Close() }()
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("source blob: %w", err)
	}
	if int64(len(data)) != obj.Size() {
		return nil, ErrMalformedDiff
	}
	return data, nil
}

func validObjectID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
