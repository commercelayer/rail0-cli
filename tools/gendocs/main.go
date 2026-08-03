// Command gendocs regenerates the rail0 CLI reference from the Cobra command
// tree: Markdown pages (for a docs site) and man pages. It lives in its own main
// package so its doc-generation dependencies (cobra/doc, md2man) are never linked
// into the shipped `rail0` binary. Invoked by bin/docs.
//
//	go run ./tools/gendocs [outdir]   # default outdir: ./docs
package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/rail0/cli/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// docsDate stamps the man pages' .TH header. PINNED, and it must stay pinned:
// with a nil header Date, Cobra fills it with time.Now() formatted "Jan 2006",
// so the committed man pages went stale BY THEMSELVES at every month boundary —
// and since CI regenerates and fails on any diff, the first pull request of a
// new month went red with 60 files changed and not a line of code touched. The
// docs check is meant to catch a stale reference, not the calendar.
//
// Pinning also overrides Cobra's SOURCE_DATE_EPOCH path (it only applies when
// Date is nil), so the output is identical whoever runs it, wherever.
//
// Bump it deliberately when the reference is meaningfully revised — a man page
// date means "when this page was last revised", which is a human judgement, not
// the wall clock at generation time.
var docsDate = time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

func main() {
	out := "docs"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}

	root := cmd.Root()
	// Drop the "Auto generated … on <date>" footer so regenerated docs have stable
	// diffs (no timestamp churn) when committed.
	disableAutoGenTag(root)

	mdDir := filepath.Join(out, "cli")
	manDir := filepath.Join(out, "man")
	for _, d := range []string{mdDir, manDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			log.Fatalf("mkdir %s: %v", d, err)
		}
	}

	if err := doc.GenMarkdownTree(root, mdDir); err != nil {
		log.Fatalf("markdown: %v", err)
	}
	hdr := &doc.GenManHeader{Title: "RAIL0", Section: "1", Source: "rail0 CLI", Date: &docsDate}
	if err := doc.GenManTree(root, hdr, manDir); err != nil {
		log.Fatalf("man: %v", err)
	}

	log.Printf("CLI docs written to %s (markdown) and %s (man pages)", mdDir, manDir)
}

// disableAutoGenTag turns off the auto-generated timestamp footer on a command and
// all of its descendants (Cobra checks the flag per command, without inheritance).
func disableAutoGenTag(c *cobra.Command) {
	c.DisableAutoGenTag = true
	for _, sub := range c.Commands() {
		disableAutoGenTag(sub)
	}
}
