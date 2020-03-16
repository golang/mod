// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package modfile

import (
	"bytes"
	"testing"

	"golang.org/x/mod/module"
)

var addRequireTests = []struct {
	desc string
	in   string
	path string
	vers string
	out  string
}{
	{
		`existing`,
		`
		module m
		require x.y/z v1.2.3
		`,
		"x.y/z", "v1.5.6",
		`
		module m
		require x.y/z v1.5.6
		`,
	},
	{
		`new`,
		`
		module m
		require x.y/z v1.2.3
		`,
		"x.y/w", "v1.5.6",
		`
		module m
		require (
			x.y/z v1.2.3
			x.y/w v1.5.6
		)
		`,
	},
	{
		`new2`,
		`
		module m
		require x.y/z v1.2.3
		require x.y/q/v2 v2.3.4
		`,
		"x.y/w", "v1.5.6",
		`
		module m
		require x.y/z v1.2.3
		require (
			x.y/q/v2 v2.3.4
			x.y/w v1.5.6
		)
		`,
	},
}

var setRequireTests = []struct {
	desc string
	in   string
	mods []struct {
		path     string
		vers     string
		indirect bool
	}
	out string
}{
	{
		`existing`,
		`module m
		require (
			x.y/b v1.2.3

			x.y/a v1.2.3
			x.y/d v1.2.3
		)
		`,
		[]struct {
			path     string
			vers     string
			indirect bool
		}{
			{"x.y/a", "v1.2.3", false},
			{"x.y/b", "v1.2.3", false},
			{"x.y/c", "v1.2.3", false},
		},
		`module m
		require (
			x.y/a v1.2.3
			x.y/b v1.2.3
			x.y/c v1.2.3
		)
		`,
	},
	{
		`existing_indirect`,
		`module m
		require (
			x.y/a v1.2.3
			x.y/b v1.2.3 //
			x.y/c v1.2.3 //c
			x.y/d v1.2.3 //   c
			x.y/e v1.2.3 // indirect
			x.y/f v1.2.3 //indirect
			x.y/g v1.2.3 //	indirect
		)
		`,
		[]struct {
			path     string
			vers     string
			indirect bool
		}{
			{"x.y/a", "v1.2.3", true},
			{"x.y/b", "v1.2.3", true},
			{"x.y/c", "v1.2.3", true},
			{"x.y/d", "v1.2.3", true},
			{"x.y/e", "v1.2.3", true},
			{"x.y/f", "v1.2.3", true},
			{"x.y/g", "v1.2.3", true},
		},
		`module m
		require (
			x.y/a v1.2.3 // indirect
			x.y/b v1.2.3 // indirect
			x.y/c v1.2.3 // indirect; c
			x.y/d v1.2.3 // indirect; c
			x.y/e v1.2.3 // indirect
			x.y/f v1.2.3 //indirect
			x.y/g v1.2.3 //	indirect
		)
		`,
	},
}

var addGoTests = []struct {
	desc    string
	in      string
	version string
	out     string
}{
	{
		`module_only`,
		`module m
		`,
		`1.14`,
		`module m
		go 1.14
		`,
	},
	{
		`module_before_require`,
		`module m
		require x.y/a v1.2.3
		`,
		`1.14`,
		`module m
		go 1.14
		require x.y/a v1.2.3
		`,
	},
	{
		`require_before_module`,
		`require x.y/a v1.2.3
		module example.com/inverted
		`,
		`1.14`,
		`require x.y/a v1.2.3
		module example.com/inverted
		go 1.14
		`,
	},
	{
		`require_only`,
		`require x.y/a v1.2.3
		`,
		`1.14`,
		`require x.y/a v1.2.3
		go 1.14
		`,
	},
}

var addRetractTests = []struct {
	desc      string
	in        string
	low       string
	high      string
	rationale string
	out       string
}{
	{
		`new_singleton`,
		`module m
		`,
		`v1.2.3`,
		`v1.2.3`,
		``,
		`module m
		retract v1.2.3
		`,
	},
	{
		`new_interval`,
		`module m
		`,
		`v1.0.0`,
		`v1.1.0`,
		``,
		`module m
		retract [v1.0.0, v1.1.0]`,
	},
	{
		`duplicate_with_rationale`,
		`module m
		retract v1.2.3
		`,
		`v1.2.3`,
		`v1.2.3`,
		`bad`,
		`module m
		retract (
			v1.2.3
			// bad
			v1.2.3
		)
		`,
	},
	{
		`duplicate_multiline_rationale`,
		`module m
		retract [v1.2.3, v1.2.3]
		`,
		`v1.2.3`,
		`v1.2.3`,
		`multi
line`,
		`module m
		retract	(
			[v1.2.3, v1.2.3]
			// multi
			// line
			v1.2.3
		)
		`,
	},
	{
		`duplicate_interval`,
		`module m
		retract [v1.0.0, v1.1.0]
		`,
		`v1.0.0`,
		`v1.1.0`,
		``,
		`module m
		retract (
			[v1.0.0, v1.1.0]
			[v1.0.0, v1.1.0]
		)
		`,
	},
	{
		`duplicate_singleton`,
		`module m
		retract v1.2.3
		`,
		`v1.2.3`,
		`v1.2.3`,
		``,
		`module m
		retract	(
			v1.2.3
			v1.2.3
		)
		`,
	},
}

var dropRetractTests = []struct {
	desc string
	in   string
	low  string
	high string
	out  string
}{
	{
		`singleton_no_match`,
		`module m
		retract v1.2.3
		`,
		`v1.0.0`,
		`v1.0.0`,
		`module m
		retract v1.2.3
		`,
	},
	{
		`singleton_match_one`,
		`module m
		retract v1.2.2
		retract v1.2.3
		retract v1.2.4
		`,
		`v1.2.3`,
		`v1.2.3`,
		`module m
		retract v1.2.2
		retract v1.2.4
		`,
	},
	{
		`singleton_match_all`,
		`module m
		retract v1.2.3 // first
		retract v1.2.3 // second
		`,
		`v1.2.3`,
		`v1.2.3`,
		`module m
		`,
	},
	{
		`interval_match`,
		`module m
		retract [v1.2.3, v1.2.3]
		`,
		`v1.2.3`,
		`v1.2.3`,
		`module m
		`,
	},
	{
		`interval_superset_no_match`,
		`module m
		retract [v1.0.0, v1.1.0]
		`,
		`v1.0.0`,
		`v1.2.0`,
		`module m
		retract [v1.0.0, v1.1.0]
		`,
	},
	{
		`singleton_match_middle`,
		`module m
		retract v1.2.3
		`,
		`v1.2.3`,
		`v1.2.3`,
		`module m
		`,
	},
	{
		`interval_match_middle_block`,
		`module m
		retract (
			v1.0.0
			[v1.1.0, v1.2.0]
			v1.3.0
		)
		`,
		`v1.1.0`,
		`v1.2.0`,
		`module m
		retract (
			v1.0.0
			v1.3.0
		)
		`,
	},
	{
		`interval_match_all`,
		`module m
		retract [v1.0.0, v1.1.0]
		retract [v1.0.0, v1.1.0]
		`,
		`v1.0.0`,
		`v1.1.0`,
		`module m
		`,
	},
}

var retractRationaleTests = []struct {
	desc, in, want string
}{
	{
		`no_comment`,
		`module m
		retract v1.0.0`,
		``,
	},
	{
		`prefix_one`,
		`module m
		//   prefix
		retract v1.0.0 
		`,
		`prefix`,
	},
	{
		`prefix_multiline`,
		`module m
		//  one
		//
		//     two
		//
		// three  
		retract v1.0.0`,
		`one

two

three`,
	},
	{
		`suffix`,
		`module m
		retract v1.0.0 // suffix
		`,
		`suffix`,
	},
	{
		`prefix_suffix_after`,
		`module m
		// prefix
		retract v1.0.0 // suffix
		`,
		`prefix
suffix`,
	},
	{
		`block_only`,
		`// block
		retract (
			v1.0.0
		)
		`,
		`block`,
	},
	{
		`block_and_line`,
		`// block
		retract (
			// line
			v1.0.0
		)
		`,
		`line`,
	},
}

var sortBlocksTests = []struct {
	desc, in, out string
	strict        bool
}{
	{
		`exclude_duplicates_removed`,
		`module m
		exclude x.y/z v1.0.0 // a
		exclude x.y/z v1.0.0 // b
		exclude (
			x.y/w v1.1.0
			x.y/z v1.0.0 // c
		)
		`,
		`module m
		exclude x.y/z v1.0.0 // a
		exclude (
			x.y/w v1.1.0
		)`,
		true,
	},
	{
		`replace_duplicates_removed`,
		`module m
		replace x.y/z v1.0.0 => ./a
		replace x.y/z v1.1.0 => ./b
		replace (
			x.y/z v1.0.0 => ./c
		)
		`,
		`module m
		replace x.y/z v1.1.0 => ./b
		replace (
			x.y/z v1.0.0 => ./c
		)
		`,
		true,
	},
	{
		`retract_duplicates_not_removed`,
		`module m
		// block
		retract (
			v1.0.0 // one
			v1.0.0 // two
		)`,
		`module m
		// block
		retract (
			v1.0.0 // one
			v1.0.0 // two
		)`,
		true,
	},
	// Tests below this point just check sort order.
	// Non-retract blocks are sorted lexicographically in ascending order.
	// retract blocks are sorted using semver in descending order.
	{
		`sort_lexicographically`,
		`module m
		sort (
			aa
			cc
			bb
			zz
			v1.2.0
			v1.11.0
		)`,
		`module m
		sort (
			aa
			bb
			cc
			v1.11.0
			v1.2.0
			zz
		)
		`,
		false,
	},
	{
		`sort_retract`,
		`module m
		retract (
			[v1.2.0, v1.3.0]
			[v1.1.0, v1.3.0]
			[v1.1.0, v1.2.0]
			v1.0.0
			v1.1.0
			v1.2.0
			v1.3.0
			v1.4.0
		)
		`,
		`module m
		retract (
			v1.4.0
			v1.3.0
			[v1.2.0, v1.3.0]
			v1.2.0
			[v1.1.0, v1.3.0]
			[v1.1.0, v1.2.0]
			v1.1.0
			v1.0.0
		)
		`,
		false,
	},
}

func TestAddRequire(t *testing.T) {
	for _, tt := range addRequireTests {
		t.Run(tt.desc, func(t *testing.T) {
			testEdit(t, tt.in, tt.out, true, func(f *File) error {
				return f.AddRequire(tt.path, tt.vers)
			})
		})
	}
}

func TestSetRequire(t *testing.T) {
	for _, tt := range setRequireTests {
		t.Run(tt.desc, func(t *testing.T) {
			var mods []*Require
			for _, mod := range tt.mods {
				mods = append(mods, &Require{
					Mod: module.Version{
						Path:    mod.path,
						Version: mod.vers,
					},
					Indirect: mod.indirect,
				})
			}

			f := testEdit(t, tt.in, tt.out, true, func(f *File) error {
				f.SetRequire(mods)
				return nil
			})

			f.Cleanup()
			if len(f.Require) != len(mods) {
				t.Errorf("after Cleanup, len(Require) = %v; want %v", len(f.Require), len(mods))
			}
		})
	}
}

func TestAddGo(t *testing.T) {
	for _, tt := range addGoTests {
		t.Run(tt.desc, func(t *testing.T) {
			testEdit(t, tt.in, tt.out, true, func(f *File) error {
				return f.AddGoStmt(tt.version)
			})
		})
	}
}

func TestAddRetract(t *testing.T) {
	for _, tt := range addRetractTests {
		t.Run(tt.desc, func(t *testing.T) {
			testEdit(t, tt.in, tt.out, true, func(f *File) error {
				return f.AddRetract(VersionInterval{Low: tt.low, High: tt.high}, tt.rationale)
			})
		})
	}
}

func TestDropRetract(t *testing.T) {
	for _, tt := range dropRetractTests {
		t.Run(tt.desc, func(t *testing.T) {
			testEdit(t, tt.in, tt.out, true, func(f *File) error {
				if err := f.DropRetract(VersionInterval{Low: tt.low, High: tt.high}); err != nil {
					return err
				}
				f.Cleanup()
				return nil
			})
		})
	}
}

func TestRetractRationale(t *testing.T) {
	for _, tt := range retractRationaleTests {
		t.Run(tt.desc, func(t *testing.T) {
			f, err := Parse("in", []byte(tt.in), nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(f.Retract) != 1 {
				t.Fatalf("got %d retract directives; want 1", len(f.Retract))
			}
			if got := f.Retract[0].Rationale; got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestSortBlocks(t *testing.T) {
	for _, tt := range sortBlocksTests {
		t.Run(tt.desc, func(t *testing.T) {
			testEdit(t, tt.in, tt.out, tt.strict, func(f *File) error {
				f.SortBlocks()
				return nil
			})
		})
	}
}

func testEdit(t *testing.T, in, want string, strict bool, transform func(f *File) error) *File {
	t.Helper()
	parse := Parse
	if !strict {
		parse = ParseLax
	}
	f, err := parse("in", []byte(in), nil)
	if err != nil {
		t.Fatal(err)
	}
	g, err := parse("out", []byte(want), nil)
	if err != nil {
		t.Fatal(err)
	}
	golden, err := g.Format()
	if err != nil {
		t.Fatal(err)
	}

	if err := transform(f); err != nil {
		t.Fatal(err)
	}
	out, err := f.Format()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, golden) {
		t.Errorf("have:\n%s\nwant:\n%s", out, golden)
	}

	return f
}
