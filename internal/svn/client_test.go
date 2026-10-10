package svn

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseStatus(t *testing.T) {
	entries, err := ParseStatus([]byte(`<status><target path=".">
<entry path="a &amp; b"><wc-status item="modified" props="none"/></entry>
<entry path="props"><wc-status item="normal" props="modified"/></entry>
<entry path="tree"><wc-status item="normal" props="none" tree-conflicted="true"/></entry>
<entry path="clean"><wc-status item="normal" props="none"/></entry>
</target></status>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[0].Path != "a & b" || entries[0].Code() != "M" || entries[1].Code() != "P" || entries[2].Code() != "C" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
	if _, err := ParseStatus([]byte(`<status>`)); err == nil {
		t.Fatal("expected malformed XML error")
	}
}

// Exercise real XML, spaced/unicode/@ paths, properties and diff without a network.
func TestLocalWorkingCopy(t *testing.T) {
	for _, name := range []string{"svn", "svnadmin"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Skipf("%s unavailable", name)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	repo, wc := filepath.Join(dir, "repo"), filepath.Join(dir, "working copy")
	run := func(work, binary string, args ...string) {
		t.Helper()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = work
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", binary, args, err, out)
		}
	}
	run(dir, "svnadmin", "create", repo)
	path := filepath.ToSlash(repo)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	repoURL := (&url.URL{Scheme: "file", Path: path}).String()
	run(dir, "svn", "checkout", repoURL, wc, "--non-interactive")
	name := "文件 @ name.txt"
	file := filepath.Join(wc, name)
	if err := os.WriteFile(file, []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(wc, "svn", "add", "--", name+"@")
	run(wc, "svn", "commit", "-m", "initial", "--non-interactive")
	if err := os.WriteFile(file, []byte("after\n"), 0600); err != nil {
		t.Fatal(err)
	}
	run(wc, "svn", "propset", "test", "value", "--", name+"@")
	if err := os.WriteFile(filepath.Join(wc, "new.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	c := Client{Dir: wc}
	info, err := c.Info(ctx)
	if err != nil || info.Root == "" || info.URL != repoURL {
		t.Fatalf("info: %+v, %v", info, err)
	}
	entries, err := c.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("status: %+v", entries)
	}
	found := false
	for _, entry := range entries {
		if entry.Path != name {
			continue
		}
		found = true
		diff, err := c.Diff(ctx, entry)
		if err != nil || !strings.Contains(diff, "-before") || !strings.Contains(diff, "+after") || !strings.Contains(diff, "Property changes") {
			t.Fatalf("diff: %v\n%s", err, diff)
		}
	}
	if !found {
		t.Fatalf("missing unicode path: %+v", entries)
	}
	working, err := c.WorkingDetail(ctx)
	if err != nil || !strings.Contains(working, "+after") || !strings.Contains(working, "new.txt") {
		t.Fatalf("working detail: %v\n%s", err, working)
	}
	run(wc, "svn", "commit", "-m", "第二次提交\n完整说明", "--non-interactive")
	commits, err := c.Log(ctx, 0, 1)
	if err != nil || len(commits) != 1 || commits[0].Revision != 2 || len(commits[0].Paths) != 0 {
		t.Fatalf("history: %+v %v", commits, err)
	}
	detail, err := c.CommitDetail(ctx, commits[0])
	if err != nil || !strings.Contains(detail, "+after") || !strings.Contains(detail, "Revision: r2") || !strings.Contains(detail, "M /") {
		t.Fatalf("commit detail: %v\n%s", err, detail)
	}
	older, err := c.Log(ctx, commits[0].Revision, 1)
	if err != nil || len(older) != 1 || older[0].Revision != 1 {
		t.Fatalf("older page: %+v %v", older, err)
	}
	last, err := c.Log(ctx, 1, 1)
	if err != nil || len(last) != 0 {
		t.Fatalf("last page: %+v %v", last, err)
	}
	run(wc, "svn", "add", "new.txt")
	run(wc, "svn", "commit", "-m", "add new", "--non-interactive")
	clean, err := c.WorkingDetail(ctx)
	if err != nil || !strings.Contains(clean, "No local changes") {
		t.Fatalf("clean: %v\n%s", err, clean)
	}
	rootInfo, err := c.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	position, err := c.Position(ctx, rootInfo)
	if err != nil || position.MinRevision == position.MaxRevision {
		t.Fatalf("expected mixed revisions: %+v %v", position, err)
	}
	run(wc, "svn", "update", "--non-interactive")
	rootInfo, err = c.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	position, err = c.Position(ctx, rootInfo)
	if rootInfo.BaseCommit != 3 {
		t.Fatalf("local info missing updated root commit: %+v", rootInfo)
	}
	if err != nil || position.BaseCommit != 3 || position.MinRevision != 3 || position.MaxRevision != 3 {
		t.Fatalf("updated position: %+v %v", position, err)
	}
	_, err = (Client{Dir: dir}).Info(ctx)
	if err == nil {
		t.Fatal("expected non-working-copy error")
	}
}
