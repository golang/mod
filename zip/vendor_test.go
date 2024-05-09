// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package zip

import "testing"

var pre124 []string = []string{
	"",
	"go1.14",
	"go1.21.0",
	"go1.22.4",
	"go1.23",
	"go1.23.1",
	"go1.2",
	"go1.7",
	"go1.9",
}

func TestIsVendoredPackage(t *testing.T) {
	for _, tc := range []struct {
		path          string
		want          bool
		falsePositive bool // is this case affected by https://golang.org/issue/37397?
		versions      []string
	}{
		{path: "vendor/foo/foo.go", want: true, versions: pre124},
		{path: "pkg/vendor/foo/foo.go", want: true, versions: pre124},
		{path: "longpackagename/vendor/foo/foo.go", want: true, versions: pre124},
		{path: "vendor/vendor.go", want: false, versions: pre124},
		{path: "vendor/foo/modules.txt", want: true, versions: pre124},
		{path: "modules.txt", want: false, versions: pre124},
		{path: "vendor/amodules.txt", want: false, versions: pre124},

		// These test cases were affected by https://golang.org/issue/63395
		{path: "vendor/modules.txt", want: false, versions: pre124},
		{path: "vendor/modules.txt", want: true, versions: []string{"go1.24.0", "go1.24", "go1.99.0"}},

		// We ideally want these cases to be false, but they are affected by
		// https://golang.org/issue/37397, and if we fix them we will invalidate
		// existing module checksums. We must leave them as-is-for now.
		{path: "pkg/vendor/vendor.go", falsePositive: true},
		{path: "longpackagename/vendor/vendor.go", falsePositive: true},
	} {
		for _, v := range tc.versions {
			got := isVendoredPackage(tc.path, v)
			want := tc.want
			if tc.falsePositive {
				want = true
			}
			if got != want {
				t.Errorf("isVendoredPackage(%q, %s) = %t; want %t", tc.path, v, got, tc.want)
			}
			if tc.falsePositive {
				t.Logf("(Expected a false-positive due to https://golang.org/issue/37397.)")
			}
		}
	}
}
