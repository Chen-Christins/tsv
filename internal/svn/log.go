package svn

import (
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type ChangedPath struct {
	Action       string `xml:"action,attr"`
	Path         string `xml:",chardata"`
	CopyFrom     string `xml:"copyfrom-path,attr"`
	CopyRevision string `xml:"copyfrom-rev,attr"`
}

type Commit struct {
	Revision int           `xml:"revision,attr"`
	Author   string        `xml:"author"`
	Date     time.Time     `xml:"date"`
	Message  string        `xml:"msg"`
	Paths    []ChangedPath `xml:"paths>path"`
}

func ParseLog(data []byte) ([]Commit, error) {
	var doc struct {
		Commits []Commit `xml:"logentry"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse svn log: %w", err)
	}
	return doc.Commits, nil
}

// before is an exclusive revision bound, or zero for HEAD.
func (c Client) Log(ctx context.Context, before, limit int) ([]Commit, error) {
	start := "HEAD"
	if before > 0 {
		start = strconv.Itoa(before - 1)
	}
	if before == 1 {
		return nil, nil
	}
	out, err := c.run(ctx, "log", "--xml", "--non-interactive", "--limit", strconv.Itoa(limit), "-r", start+":1", "--", ".")
	if err != nil {
		return nil, err
	}
	return ParseLog(out)
}

func (c Client) WorkingDetail(ctx context.Context) (string, error) {
	entries, err := c.Status(ctx)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("Not Committed Yet\nUnstaged changes\n\n")
	if len(entries) == 0 {
		b.WriteString("No local changes.\n")
		return b.String(), nil
	}
	for _, e := range entries {
		fmt.Fprintf(&b, "%s %s\n", e.Code(), e.Path)
	}
	out, err := c.run(ctx, "diff", "--internal-diff", "--non-interactive", "--", ".")
	if err != nil {
		return "", err
	}
	b.WriteString("\n")
	b.Write(out)
	b.WriteString("\nUnversioned files are listed above but are not included in SVN diff.\n")
	return strings.ReplaceAll(b.String(), "\r\n", "\n"), nil
}

func (c Client) CommitDetail(ctx context.Context, commit Commit) (string, error) {
	info, err := c.Info(ctx)
	if err != nil {
		return "", err
	}
	target := info.URL + "@" + strconv.Itoa(commit.Revision)
	metadata, err := c.run(ctx, "log", "--xml", "--verbose", "--non-interactive", "-r", strconv.Itoa(commit.Revision), "--", target)
	if err != nil {
		return "", err
	}
	commits, err := ParseLog(metadata)
	if err != nil {
		return "", err
	}
	if len(commits) > 0 {
		commit = commits[0]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Revision: r%d\nAuthor: %s\nDate: %s\n\n%s\n\nChanged paths:\n", commit.Revision, commit.Author, commit.Date.Local().Format("2006-01-02 15:04:05 -0700"), commit.Message)
	for _, p := range commit.Paths {
		fmt.Fprintf(&b, "%s %s", p.Action, p.Path)
		if p.CopyFrom != "" {
			fmt.Fprintf(&b, " (from %s@%s)", p.CopyFrom, p.CopyRevision)
		}
		b.WriteByte('\n')
	}
	// Resolve at the selected revision rather than at the working copy's BASE.
	out, err := c.run(ctx, "diff", "--internal-diff", "--non-interactive", "-c", strconv.Itoa(commit.Revision), "--", info.URL+"@"+strconv.Itoa(commit.Revision))
	if err != nil {
		return b.String() + "\n", err
	}
	b.WriteString("\n")
	b.Write(out)
	return strings.ReplaceAll(b.String(), "\r\n", "\n"), nil
}
