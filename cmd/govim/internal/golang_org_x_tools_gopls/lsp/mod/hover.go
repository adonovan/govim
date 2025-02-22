// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mod

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"golang.org/x/mod/modfile"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/event"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/source"
)

func Hover(ctx context.Context, snapshot source.Snapshot, fh source.FileHandle, position protocol.Position) (*protocol.Hover, error) {
	var found bool
	for _, uri := range snapshot.ModFiles() {
		if fh.URI() == uri {
			found = true
			break
		}
	}

	// We only provide hover information for the view's go.mod files.
	if !found {
		return nil, nil
	}

	ctx, done := event.Start(ctx, "mod.Hover")
	defer done()

	// Get the position of the cursor.
	pm, err := snapshot.ParseMod(ctx, fh)
	if err != nil {
		return nil, fmt.Errorf("getting modfile handle: %w", err)
	}
	offset, err := pm.Mapper.Offset(position)
	if err != nil {
		return nil, fmt.Errorf("computing cursor position: %w", err)
	}

	// Confirm that the cursor is at the position of a require statement.
	var req *modfile.Require
	var startPos, endPos int
	for _, r := range pm.File.Require {
		dep := []byte(r.Mod.Path)
		s, e := r.Syntax.Start.Byte, r.Syntax.End.Byte
		i := bytes.Index(pm.Mapper.Content[s:e], dep)
		if i == -1 {
			continue
		}
		// Shift the start position to the location of the
		// dependency within the require statement.
		startPos, endPos = s+i, s+i+len(dep)
		if startPos <= offset && offset <= endPos {
			req = r
			break
		}
	}

	// The cursor position is not on a require statement.
	if req == nil {
		return nil, nil
	}

	// Get the `go mod why` results for the given file.
	why, err := snapshot.ModWhy(ctx, fh)
	if err != nil {
		return nil, err
	}
	explanation, ok := why[req.Mod.Path]
	if !ok {
		return nil, nil
	}

	// Get the range to highlight for the hover.
	rng, err := source.ByteOffsetsToRange(pm.Mapper, fh.URI(), startPos, endPos)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	options := snapshot.View().Options()
	isPrivate := snapshot.View().IsGoPrivatePath(req.Mod.Path)
	explanation = formatExplanation(explanation, req, options, isPrivate)
	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  options.PreferredContentFormat,
			Value: explanation,
		},
		Range: rng,
	}, nil
}

func formatExplanation(text string, req *modfile.Require, options *source.Options, isPrivate bool) string {
	text = strings.TrimSuffix(text, "\n")
	splt := strings.Split(text, "\n")
	length := len(splt)

	var b strings.Builder
	// Write the heading as an H3.
	b.WriteString("##" + splt[0])
	if options.PreferredContentFormat == protocol.Markdown {
		b.WriteString("\n\n")
	} else {
		b.WriteRune('\n')
	}

	// If the explanation is 2 lines, then it is of the form:
	// # golang.org/x/text/encoding
	// (main module does not need package golang.org/x/text/encoding)
	if length == 2 {
		b.WriteString(splt[1])
		return b.String()
	}

	imp := splt[length-1] // import path
	reference := imp
	// See golang/go#36998: don't link to modules matching GOPRIVATE.
	if !isPrivate && options.PreferredContentFormat == protocol.Markdown {
		target := imp
		if strings.ToLower(options.LinkTarget) == "pkg.go.dev" {
			target = strings.Replace(target, req.Mod.Path, req.Mod.String(), 1)
		}
		reference = fmt.Sprintf("[%s](%s)", imp, source.BuildLink(options.LinkTarget, target, ""))
	}
	b.WriteString("This module is necessary because " + reference + " is imported in")

	// If the explanation is 3 lines, then it is of the form:
	// # golang.org/x/tools
	// modtest
	// golang.org/x/tools/go/packages
	if length == 3 {
		msg := fmt.Sprintf(" `%s`.", splt[1])
		b.WriteString(msg)
		return b.String()
	}

	// If the explanation is more than 3 lines, then it is of the form:
	// # golang.org/x/text/language
	// rsc.io/quote
	// rsc.io/sampler
	// golang.org/x/text/language
	b.WriteString(":\n```text")
	dash := ""
	for _, imp := range splt[1 : length-1] {
		dash += "-"
		b.WriteString("\n" + dash + " " + imp)
	}
	b.WriteString("\n```")
	return b.String()
}
