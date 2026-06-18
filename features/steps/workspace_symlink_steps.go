package steps

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/cucumber/godog"
	messages "github.com/cucumber/messages/go/v21"

	"github.com/hotchkj/glyph-shift/internal/fileops"
	"github.com/hotchkj/glyph-shift/internal/fsnorm"
	"github.com/hotchkj/glyph-shift/internal/pipeline"
	"github.com/hotchkj/glyph-shift/internal/testutil"
	"github.com/hotchkj/glyph-shift/internal/validate"
)

const (
	workspaceSymlinkScopeInside  = "inside"
	workspaceSymlinkScopeOutside = "outside"
	workspaceSymlinkKindFile     = "file"
	workspaceSymlinkKindDir      = "directory"
	workspaceSymlinkQuotedWrap   = "%w: %q"
)

var (
	errWorkspaceSymlinkMapRows        = errors.New("workspace symlink map must include a header and at least one row")
	errWorkspaceSymlinkTargetScope    = errors.New("workspace symlink target_scope must be inside or outside")
	errWorkspaceSymlinkTargetKind     = errors.New("workspace symlink target_kind must be file or directory")
	errWorkspaceSymlinkUnexpectedCell = errors.New("workspace symlink map has unexpected cell")
	errWorkspaceSymlinkMissingColumn  = errors.New("workspace symlink map missing required column")
)

type workspaceSymlink struct {
	linkAbs     string
	targetAbs   string
	targetScope string
	targetKind  string
}

func registerWorkspaceSymlinkMap(sc *godog.ScenarioContext, tc *TestContext) {
	sc.Given(`^the workspace symlink map:$`, func(table *godog.Table) error {
		return tc.applyWorkspaceSymlinkMap(table)
	})
}

func (tc *TestContext) applyWorkspaceSymlinkMap(table *godog.Table) error {
	if table == nil || len(table.Rows) < 2 {
		return errWorkspaceSymlinkMapRows
	}

	headers := godogTableHeaders(table.Rows[0])
	for _, row := range table.Rows[1:] {
		record, err := godogTableRecord(headers, row)
		if err != nil {
			return err
		}

		link, err := tc.workspaceSymlinkFromRecord(record)
		if err != nil {
			return err
		}
		tc.WorkspaceSymlinks[link.linkAbs] = link
	}

	return nil
}

func (tc *TestContext) workspaceSymlinkFromRecord(record map[string]string) (workspaceSymlink, error) {
	linkRel := record["link"]
	targetScope := record["target_scope"]
	targetRel := record["target"]
	targetKind := record["target_kind"]

	if targetScope != workspaceSymlinkScopeInside && targetScope != workspaceSymlinkScopeOutside {
		return workspaceSymlink{}, fmt.Errorf(workspaceSymlinkQuotedWrap, errWorkspaceSymlinkTargetScope, targetScope)
	}
	if targetKind != workspaceSymlinkKindFile && targetKind != workspaceSymlinkKindDir {
		return workspaceSymlink{}, fmt.Errorf(workspaceSymlinkQuotedWrap, errWorkspaceSymlinkTargetKind, targetKind)
	}

	root := fsnorm.DirNative(tc.Ws.Root())
	linkAbs := fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(linkRel), root)
	targetAbs := tc.workspaceSymlinkTargetAbs(targetScope, targetRel)

	return workspaceSymlink{
		linkAbs:     filepath.Clean(linkAbs),
		targetAbs:   filepath.Clean(targetAbs),
		targetScope: targetScope,
		targetKind:  targetKind,
	}, nil
}

func (tc *TestContext) workspaceSymlinkTargetAbs(targetScope, targetRel string) string {
	root := fsnorm.DirNative(tc.Ws.Root())
	if targetScope == workspaceSymlinkScopeInside {
		return fsnorm.ResolveUnderWorkspace(fsnorm.Canonical(targetRel), root)
	}

	outsideRoot := filepath.Join(filepath.Dir(root), ".outside-workspace")

	return filepath.Join(outsideRoot, filepath.FromSlash(targetRel))
}

func godogTableHeaders(row *messages.PickleTableRow) map[int]string {
	headers := make(map[int]string, len(row.Cells))
	for idx, cell := range row.Cells {
		headers[idx] = strings.TrimSpace(cell.Value)
	}

	return headers
}

func godogTableRecord(headers map[int]string, row *messages.PickleTableRow) (map[string]string, error) {
	record := make(map[string]string, len(headers))
	for idx, cell := range row.Cells {
		header, ok := headers[idx]
		if !ok {
			return nil, fmt.Errorf("%w: index %d", errWorkspaceSymlinkUnexpectedCell, idx)
		}
		record[header] = strings.TrimSpace(cell.Value)
	}

	for _, required := range []string{"link", "target_scope", "target", "target_kind"} {
		if record[required] == "" {
			return nil, fmt.Errorf(workspaceSymlinkQuotedWrap, errWorkspaceSymlinkMissingColumn, required)
		}
	}

	return record, nil
}

func (tc *TestContext) symlinkAwareResolver(base validate.PathResolver) validate.PathResolver {
	if len(tc.WorkspaceSymlinks) == 0 {
		return base
	}

	return workspaceSymlinkResolver{base: base, links: tc.WorkspaceSymlinks}
}

func (tc *TestContext) symlinkAwareSourceOpener(base pipeline.SourceOpener) pipeline.SourceOpener {
	if len(tc.WorkspaceSymlinks) == 0 {
		return base
	}

	return workspaceSymlinkSourceOpener{base: base, links: tc.WorkspaceSymlinks}
}

func (tc *TestContext) symlinkAwareFileStater(base pipeline.FileStater) pipeline.FileStater {
	if len(tc.WorkspaceSymlinks) == 0 {
		return base
	}

	return workspaceSymlinkFileStater{base: base, links: tc.WorkspaceSymlinks}
}

func (tc *TestContext) symlinkAwareFileSession(base fileops.FileSession) fileops.FileSession {
	if len(tc.WorkspaceSymlinks) == 0 {
		return base
	}

	return testutil.NewPathRewritingFileSession(base, func(path string) string {
		return workspaceSymlinkInsideFileTarget(tc.WorkspaceSymlinks, path)
	})
}

type workspaceSymlinkResolver struct {
	base  validate.PathResolver
	links map[string]workspaceSymlink
}

func (r workspaceSymlinkResolver) Lstat(path string) (fs.FileInfo, error) {
	if link, ok := workspaceSymlinkForExactPath(r.links, path); ok {
		return workspaceSymlinkInfo{mode: fs.ModeSymlink, name: filepath.Base(link.linkAbs)}, nil
	}

	return r.base.Lstat(path)
}

func (r workspaceSymlinkResolver) EvalSymlinks(path string) (string, error) {
	if link, ok := workspaceSymlinkForExactPath(r.links, path); ok {
		return link.targetAbs, nil
	}

	return r.base.EvalSymlinks(path)
}

type workspaceSymlinkSourceOpener struct {
	base  pipeline.SourceOpener
	links map[string]workspaceSymlink
}

func (o workspaceSymlinkSourceOpener) Open(path string) (io.ReadSeekCloser, error) {
	return o.base.Open(workspaceSymlinkInsideFileTarget(o.links, path))
}

type workspaceSymlinkFileStater struct {
	base  pipeline.FileStater
	links map[string]workspaceSymlink
}

func (s workspaceSymlinkFileStater) Stat(path string) (fs.FileInfo, error) {
	link, ok := workspaceSymlinkForExactPath(s.links, path)
	if !ok {
		return s.base.Stat(path)
	}

	if link.targetScope == workspaceSymlinkScopeInside {
		return s.base.Stat(link.targetAbs)
	}

	if link.targetKind == workspaceSymlinkKindDir {
		return workspaceSymlinkInfo{mode: fs.ModeDir, name: filepath.Base(link.linkAbs)}, nil
	}

	return workspaceSymlinkInfo{name: filepath.Base(link.linkAbs)}, nil
}

type workspaceSymlinkInfo struct {
	mode fs.FileMode
	name string
}

func (i workspaceSymlinkInfo) Name() string {
	return i.name
}

func (i workspaceSymlinkInfo) Size() int64 {
	return 0
}

func (i workspaceSymlinkInfo) Mode() fs.FileMode {
	return i.mode
}

func (i workspaceSymlinkInfo) ModTime() time.Time {
	return time.Time{}
}

func (i workspaceSymlinkInfo) IsDir() bool {
	return i.mode.IsDir()
}

func (i workspaceSymlinkInfo) Sys() any {
	return nil
}

func workspaceSymlinkForExactPath(links map[string]workspaceSymlink, path string) (workspaceSymlink, bool) {
	link, ok := links[filepath.Clean(path)]

	return link, ok
}

func workspaceSymlinkInsideFileTarget(links map[string]workspaceSymlink, path string) string {
	link, ok := workspaceSymlinkForExactPath(links, path)
	if !ok || link.targetScope != workspaceSymlinkScopeInside || link.targetKind != workspaceSymlinkKindFile {
		return path
	}

	return link.targetAbs
}
