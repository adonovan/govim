// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmd

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/diff"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/source"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/tool"
)

// imports implements the import verb for gopls.
type imports struct {
	Diff  bool `flag:"d,diff" help:"display diffs instead of rewriting files"`
	Write bool `flag:"w,write" help:"write result to (source) file instead of stdout"`

	app *Application
}

func (t *imports) Name() string      { return "imports" }
func (t *imports) Parent() string    { return t.app.Name() }
func (t *imports) Usage() string     { return "[imports-flags] <filename>" }
func (t *imports) ShortHelp() string { return "updates import statements" }
func (t *imports) DetailedHelp(f *flag.FlagSet) {
	fmt.Fprintf(f.Output(), `
Example: update imports statements in a file:

	$ gopls imports -w internal/lsp/cmd/check.go

imports-flags:
`)
	printFlagDefaults(f)
}

// Run performs diagnostic checks on the file specified and either;
// - if -w is specified, updates the file in place;
// - if -d is specified, prints out unified diffs of the changes; or
// - otherwise, prints the new versions to stdout.
func (t *imports) Run(ctx context.Context, args ...string) error {
	if len(args) != 1 {
		return tool.CommandLineErrorf("imports expects 1 argument")
	}
	conn, err := t.app.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.terminate(ctx)

	from := span.Parse(args[0])
	uri := from.URI()
	file := conn.AddFile(ctx, uri)
	if file.err != nil {
		return file.err
	}
	actions, err := conn.CodeAction(ctx, &protocol.CodeActionParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: protocol.URIFromSpanURI(uri),
		},
	})
	if err != nil {
		return fmt.Errorf("%v: %v", from, err)
	}
	var edits []protocol.TextEdit
	for _, a := range actions {
		if a.Title != "Organize Imports" {
			continue
		}
		for _, c := range a.Edit.DocumentChanges {
			if c.TextDocumentEdit != nil {
				if fileURI(c.TextDocumentEdit.TextDocument.URI) == uri {
					edits = append(edits, c.TextDocumentEdit.Edits...)
				}
			}
		}
	}
	sedits, err := source.FromProtocolEdits(file.mapper, edits)
	if err != nil {
		return fmt.Errorf("%v: %v", edits, err)
	}
	newContent := diff.ApplyEdits(string(file.mapper.Content), sedits)

	filename := file.uri.Filename()
	switch {
	case t.Write:
		if len(edits) > 0 {
			ioutil.WriteFile(filename, []byte(newContent), 0644)
		}
	case t.Diff:
		diffs := diff.ToUnified(filename+".orig", filename, string(file.mapper.Content), sedits)
		fmt.Print(diffs)
	default:
		fmt.Print(string(newContent))
	}
	return nil
}
