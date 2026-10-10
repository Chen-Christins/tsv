package svn

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client runs SVN directly, without a shell, in a working-copy directory.
type Client struct{ Dir string }

type Info struct {
	Root        string
	URL         string
	Revision    string
	MinRevision int
	MaxRevision int
	BaseCommit  int
}

type Entry struct {
	Path         string
	Item         string
	Props        string
	TreeConflict bool
}

func (e Entry) Code() string {
	if e.TreeConflict || e.Item == "conflicted" || e.Props == "conflicted" {
		return "C"
	}
	codes := map[string]string{"modified": "M", "added": "A", "deleted": "D", "unversioned": "?", "missing": "!", "replaced": "R", "obstructed": "~", "external": "X", "incomplete": "!"}
	if code, ok := codes[e.Item]; ok {
		return code
	}
	if e.Props == "modified" {
		return "P"
	}
	return " "
}

func (c Client) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "svn", args...)
	cmd.Dir = c.Dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("svn %s: %w\n%s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	// XML callers need clean stdout; diff callers retain successful warnings.
	if args[0] == "diff" && stderr.Len() > 0 {
		out = append(append([]byte(stderr.String()+"\n"), out...), '\n')
	}
	return out, nil
}

func (c Client) Info(ctx context.Context) (Info, error) {
	out, err := c.run(ctx, "info", "--xml", "--non-interactive", "--", ".")
	if err != nil {
		return Info{}, err
	}
	var doc struct {
		Entry struct {
			Revision string `xml:"revision,attr"`
			URL      string `xml:"url"`
			Root     string `xml:"wc-info>wcroot-abspath"`
			Changed  struct {
				Revision int `xml:"revision,attr"`
			} `xml:"commit"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(out, &doc); err != nil {
		return Info{}, fmt.Errorf("parse svn info: %w", err)
	}
	return Info{Root: doc.Entry.Root, URL: doc.Entry.URL, Revision: doc.Entry.Revision, BaseCommit: doc.Entry.Changed.Revision}, nil
}

func ParseStatus(data []byte) ([]Entry, error) {
	var doc struct {
		Targets []struct {
			Entries []struct {
				Path   string `xml:"path,attr"`
				Status struct {
					Item         string `xml:"item,attr"`
					Props        string `xml:"props,attr"`
					TreeConflict bool   `xml:"tree-conflicted,attr"`
				} `xml:"wc-status"`
			} `xml:"entry"`
		} `xml:"target"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse svn status: %w", err)
	}
	var entries []Entry
	for _, target := range doc.Targets {
		for _, e := range target.Entries {
			if (e.Status.Item == "normal" || e.Status.Item == "none") && e.Status.Props != "modified" && e.Status.Props != "conflicted" && !e.Status.TreeConflict {
				continue
			}
			entries = append(entries, Entry{e.Path, e.Status.Item, e.Status.Props, e.Status.TreeConflict})
		}
	}
	return entries, nil
}

func (c Client) Status(ctx context.Context) ([]Entry, error) {
	out, err := c.run(ctx, "status", "--xml", "--non-interactive", "--", ".")
	if err != nil {
		return nil, err
	}
	return ParseStatus(out)
}

func (c Client) Diff(ctx context.Context, e Entry) (string, error) {
	if e.Item == "unversioned" || e.Item == "external" || e.Item == "obstructed" || e.Item == "incomplete" {
		return fmt.Sprintf("%s\n\nSVN status: %s\nNo versioned diff available.", e.Path, e.Item), nil
	}
	// --old explicitly uses peg-revision parsing even for a local-only diff.
	path := filepath.Clean(e.Path) + "@"
	out, err := c.run(ctx, "diff", "--internal-diff", "--non-interactive", "--old", path)
	if err != nil {
		return "", err
	}
	if len(out) == 0 {
		return "No text diff available (the change may be metadata-only or binary).", nil
	}
	return strings.ReplaceAll(string(out), "\r\n", "\n"), nil
}
