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

	"github.com/rail0/cli/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

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
	hdr := &doc.GenManHeader{Title: "RAIL0", Section: "1", Source: "rail0 CLI"}
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
