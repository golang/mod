// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package modfile

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

// TODO(#45713): Update these tests once AddDirectory sets the module path.
var workAddDirectoryTests = []struct {
	desc       string
	in         string
	path       string
	modulePath string
	out        string
}{
	{
		`empty`,
		``,
		`foo`, `bar`,
		`directory foo`,
	},
	{
		`go_stmt_only`,
		`go 1.17
		`,
		`foo`, `bar`,
		`go 1.17
		directory foo
		`,
	},
	{
		`directory_line_present`,
		`go 1.17
		directory baz`,
		`foo`, `bar`,
		`go 1.17
		directory (
			baz
		  foo
		)
		`,
	},
	{
		`directory_block_present`,
		`go 1.17
		directory (
			baz
			quux
		)
		`,
		`foo`, `bar`,
		`go 1.17
		directory (
			baz
		  quux
			foo
		)
		`,
	},
	{
		`directory_and_replace_present`,
		`go 1.17
		directory baz
		replace a => ./b
		`,
		`foo`, `bar`,
		`go 1.17
		directory (
			baz
			foo
		)
		replace a => ./b
		`,
	},
}

var workDropDirectoryTests = []struct {
	desc string
	in   string
	path string
	out  string
}{
	{
		`empty`,
		``,
		`foo`,
		``,
	},
	{
		`go_stmt_only`,
		`go 1.17
		`,
		`foo`,
		`go 1.17
		`,
	},
	{
		`singled_directory`,
		`go 1.17
		directory foo`,
		`foo`,
		`go 1.17
		`,
	},
	{
		`directory_block`,
		`go 1.17
		directory (
			foo
			bar
			baz
		)`,
		`bar`,
		`go 1.17
		directory (
			foo
			baz
		)`,
	},
	{
		`directory_multi`,
		`go 1.17
		directory (
			foo
			bar
			baz
		)
		directory foo
		directory quux
		directory foo`,
		`foo`,
		`go 1.17
		directory (
			bar
			baz
		)
		directory quux`,
	},
}

var workAddGoTests = []struct {
	desc    string
	in      string
	version string
	out     string
}{
	{
		`empty`,
		``,
		`1.17`,
		`go 1.17
		`,
	},
	{
		`comment`,
		`// this is a comment`,
		`1.17`,
		`// this is a comment

		go 1.17`,
	},
	{
		`directory_after_replace`,
		`
		replace example.com/foo => ../bar
		directory foo
		`,
		`1.17`,
		`
		go 1.17
		replace example.com/foo => ../bar
		directory foo
		`,
	},
	{
		`directory_before_replace`,
		`directory foo
		replace example.com/foo => ../bar
		`,
		`1.17`,
		`
		go 1.17
		directory foo
		replace example.com/foo => ../bar
		`,
	},
	{
		`directory_only`,
		`directory foo
		`,
		`1.17`,
		`
		go 1.17
		directory foo
		`,
	},
	{
		`already_have_go`,
		`go 1.17
		`,
		`1.18`,
		`
		go 1.18
		`,
	},
}

var workSortBlocksTests = []struct {
	desc, in, out string
}{
	{
		`directory_duplicates_not_removed`,
		`go 1.17
		directory foo
		directory bar
		directory (
			foo
		)`,
		`go 1.17
		directory foo
		directory bar
		directory (
			foo
		)`,
	},
	{
		`replace_duplicates_removed`,
		`go 1.17
		directory foo
		replace x.y/z v1.0.0 => ./a
		replace x.y/z v1.1.0 => ./b
		replace (
			x.y/z v1.0.0 => ./c
		)
		`,
		`go 1.17
		directory foo
		replace x.y/z v1.1.0 => ./b
		replace (
			x.y/z v1.0.0 => ./c
		)
		`,
	},
}

func TestAddDirectory(t *testing.T) {
	for _, tt := range workAddDirectoryTests {
		t.Run(tt.desc, func(t *testing.T) {
			testWorkEdit(t, tt.in, tt.out, func(f *WorkFile) error {
				return f.AddDirectory(tt.path, tt.modulePath)
			})
		})
	}
}

func TestDropDirectory(t *testing.T) {
	for _, tt := range workDropDirectoryTests {
		t.Run(tt.desc, func(t *testing.T) {
			testWorkEdit(t, tt.in, tt.out, func(f *WorkFile) error {
				if err := f.DropDirectory(tt.path); err != nil {
					return err
				}
				f.Cleanup()
				return nil
			})
		})
	}
}

func TestWorkAddGo(t *testing.T) {
	for _, tt := range workAddGoTests {
		t.Run(tt.desc, func(t *testing.T) {
			testWorkEdit(t, tt.in, tt.out, func(f *WorkFile) error {
				return f.AddGoStmt(tt.version)
			})
		})
	}
}

func TestWorkSortBlocks(t *testing.T) {
	for _, tt := range workSortBlocksTests {
		t.Run(tt.desc, func(t *testing.T) {
			testWorkEdit(t, tt.in, tt.out, func(f *WorkFile) error {
				f.SortBlocks()
				return nil
			})
		})
	}
}

// Test that when files in the testdata directory are parsed
// and printed and parsed again, we get the same parse tree
// both times.
func TestWorkPrintParse(t *testing.T) {
	outs, err := filepath.Glob("testdata/work/*")
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range outs {
		out := out
		name := filepath.Base(out)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data, err := ioutil.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}

			base := "testdata/work/" + filepath.Base(out)
			f, err := parse(base, data)
			if err != nil {
				t.Fatalf("parsing original: %v", err)
			}

			ndata := Format(f)
			f2, err := parse(base, ndata)
			if err != nil {
				t.Fatalf("parsing reformatted: %v", err)
			}

			eq := eqchecker{file: base}
			if err := eq.check(f, f2); err != nil {
				t.Errorf("not equal (parse/Format/parse): %v", err)
			}

			pf1, err := ParseWork(base, data, nil)
			if err != nil {
				switch base {
				case "testdata/replace2.in", "testdata/gopkg.in.golden":
					t.Errorf("should parse %v: %v", base, err)
				}
			}
			if err == nil {
				pf2, err := ParseWork(base, ndata, nil)
				if err != nil {
					t.Fatalf("Parsing reformatted: %v", err)
				}
				eq := eqchecker{file: base}
				if err := eq.check(pf1, pf2); err != nil {
					t.Errorf("not equal (parse/Format/Parse): %v", err)
				}

				ndata2 := Format(pf1.Syntax)
				pf3, err := ParseWork(base, ndata2, nil)
				if err != nil {
					t.Fatalf("Parsing reformatted2: %v", err)
				}
				eq = eqchecker{file: base}
				if err := eq.check(pf1, pf3); err != nil {
					t.Errorf("not equal (Parse/Format/Parse): %v", err)
				}
				ndata = ndata2
			}

			if strings.HasSuffix(out, ".in") {
				golden, err := ioutil.ReadFile(strings.TrimSuffix(out, ".in") + ".golden")
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(ndata, golden) {
					t.Errorf("formatted %s incorrectly: diff shows -golden, +ours", base)
					tdiff(t, string(golden), string(ndata))
					return
				}
			}
		})
	}
}

func testWorkEdit(t *testing.T, in, want string, transform func(f *WorkFile) error) *WorkFile {
	t.Helper()
	parse := ParseWork
	f, err := parse("in", []byte(in), nil)
	if err != nil {
		t.Fatal(err)
	}
	g, err := parse("out", []byte(want), nil)
	if err != nil {
		t.Fatal(err)
	}
	golden := Format(g.Syntax)

	if err := transform(f); err != nil {
		t.Fatal(err)
	}
	out := Format(f.Syntax)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, golden) {
		t.Errorf("have:\n%s\nwant:\n%s", out, golden)
	}

	return f
}
