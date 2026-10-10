package svn

import (
	"context"
	"encoding/xml"
	"strconv"
)

// Position records the versioned-node range and the most recent project commit
// visible at the working-copy root's BASE, which may not itself be a commit.
func (c Client) Position(ctx context.Context, info Info) (Info, error) {
	out, err := c.run(ctx, "status", "--xml", "--verbose", "--ignore-externals", "--non-interactive", "--", ".")
	if err != nil {
		return info, err
	}
	var doc struct {
		Targets []struct {
			Entries []struct {
				Status struct {
					Revision string `xml:"revision,attr"`
				} `xml:"wc-status"`
			} `xml:"entry"`
		} `xml:"target"`
	}
	if err := xml.Unmarshal(out, &doc); err != nil {
		return info, err
	}
	for _, target := range doc.Targets {
		for _, entry := range target.Entries {
			r, err := strconv.Atoi(entry.Status.Revision)
			if err != nil || r < 0 {
				continue
			}
			if info.MinRevision == 0 || r < info.MinRevision {
				info.MinRevision = r
			}
			if r > info.MaxRevision {
				info.MaxRevision = r
			}
		}
	}
	base, err := strconv.Atoi(info.Revision)
	if err != nil || base == 0 {
		return info, err
	}
	commits, err := c.Log(ctx, base+1, 1)
	if err != nil {
		return info, err
	}
	if len(commits) > 0 {
		info.BaseCommit = commits[0].Revision
	}
	return info, nil
}
