package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"tsv/internal/svn"
	"tsv/internal/tui"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: tsv [path]\n\nTig-style SVN history and working-copy diff viewer.")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	path := "."
	if flag.NArg() == 1 {
		path = flag.Arg(0)
	}
	dir, err := filepath.Abs(path)
	if err != nil {
		fail(err)
	}
	stat, err := os.Stat(dir)
	if err != nil {
		fail(err)
	}
	if !stat.IsDir() {
		fail(fmt.Errorf("path must be a directory: %s", dir))
	}
	if _, err := exec.LookPath("svn"); err != nil {
		fail(fmt.Errorf("svn is required on PATH: %w", err))
	}
	if _, err := tea.NewProgram(tui.New(svn.Client{Dir: dir}), tea.WithAltScreen()).Run(); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, "tsv:", err); os.Exit(1) }
