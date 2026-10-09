// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tlog

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCertificateTransparency(t *testing.T) {
	// Test that we can verify actual Certificate Transparency proofs.
	// (The other tests check that we can verify our own proofs;
	// this is a test that the two are compatible.)
	//
	// The testdata files are responses from the Google Argon 2020 log
	// (https://ct.googleapis.com/logs/argon2020/ct/v1/) to the
	// get-sth, get-entries?start=10000&end=10000, get-proof-by-hash,
	// and get-sth-consistency?first=3654490 endpoints, with the unused
	// extra_data field removed from the get-entries response.

	var root ctTree
	readJSON(t, "testdata/argon2020/sth.json", &root)

	var leaf ctEntries
	readJSON(t, "testdata/argon2020/entries.json", &leaf)
	hash := RecordHash(leaf.Entries[0].Data)

	var rp ctRecordProof
	readJSON(t, "testdata/argon2020/proof.json", &rp)

	err := CheckRecord(rp.Proof, root.Size, root.Hash, 10000, hash)
	if err != nil {
		t.Fatal(err)
	}

	var tp ctTreeProof
	readJSON(t, "testdata/argon2020/consistency.json", &tp)

	oh, _ := ParseHash("AuIZ5V6sDUj1vn3Y1K85oOaQ7y+FJJKtyRTl1edIKBQ=")
	err = CheckTree(tp.Proof, root.Size, root.Hash, 3654490, oh)
	if err != nil {
		t.Fatal(err)
	}
}

type ctTree struct {
	Size int64 `json:"tree_size"`
	Hash Hash  `json:"sha256_root_hash"`
}

type ctEntries struct {
	Entries []*ctEntry
}

type ctEntry struct {
	Data []byte `json:"leaf_input"`
}

type ctRecordProof struct {
	Index int64       `json:"leaf_index"`
	Proof RecordProof `json:"audit_path"`
}

type ctTreeProof struct {
	Proof TreeProof `json:"consistency"`
}

func readJSON(t *testing.T, name string, targ any) {
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, targ); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}
