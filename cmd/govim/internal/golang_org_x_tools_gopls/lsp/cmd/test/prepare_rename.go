// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmdtest

import (
	"fmt"
	"testing"

	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/cmd"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/protocol"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools_gopls/lsp/source"
	"github.com/govim/govim/cmd/govim/internal/golang_org_x_tools/span"
)

func (r *runner) PrepareRename(t *testing.T, src span.Span, want *source.PrepareItem) {
	m, err := r.data.Mapper(src.URI())
	if err != nil {
		t.Errorf("prepare_rename failed: %v", err)
	}

	var (
		target         = fmt.Sprintf("%v", src)
		args           = []string{"prepare_rename", target}
		stdOut, stdErr = r.NormalizeGoplsCmd(t, args...)
		expect         string
	)

	if want.Text == "" {
		if stdErr != "" && stdErr != cmd.ErrInvalidRenamePosition.Error() {
			t.Errorf("prepare_rename failed for %s,\nexpected:\n`%v`\ngot:\n`%v`", target, expect, stdErr)
		}
		return
	}

	ws, err := m.Span(protocol.Location{Range: want.Range})
	if err != nil {
		t.Errorf("prepare_rename failed: %v", err)
	}

	expect = r.Normalize(fmt.Sprintln(ws))
	if expect != stdOut {
		t.Errorf("prepare_rename failed for %s expected:\n`%s`\ngot:\n`%s`\n", target, expect, stdOut)
	}
}
