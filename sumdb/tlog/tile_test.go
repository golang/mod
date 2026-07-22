// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tlog

import (
	"bytes"
	"fmt"
	"testing"
)

type testTree struct {
	t      *testing.T
	h      int
	n      int64
	hashes testHashStorage
}

func newTestTree(t *testing.T, height int) *testTree {
	return &testTree{t: t, h: height}
}

// Add appends a record with the given data to the tree.
func (tt *testTree) Add(data []byte) {
	tt.t.Helper()
	hashes, err := StoredHashes(tt.n, data, tt.hashes)
	if err != nil {
		tt.t.Fatal(err)
	}
	tt.hashes = append(tt.hashes, hashes...)
	tt.n++
}

func (tt *testTree) Tree() Tree {
	tt.t.Helper()
	th, err := TreeHash(tt.n, tt.hashes)
	if err != nil {
		tt.t.Fatal(err)
	}
	return Tree{N: tt.n, Hash: th}
}

func (tt *testTree) Height() int { return tt.h }

func (tt *testTree) ReadTiles(tiles []Tile) ([][]byte, error) {
	out := make([][]byte, len(tiles))
	for i, tile := range tiles {
		data, err := ReadTileData(tile, tt.hashes)
		if err != nil {
			return nil, err
		}
		out[i] = data
	}
	return out, nil
}

func (tt *testTree) SaveTiles(tiles []Tile, data [][]byte) {
	tt.t.Helper()
	if len(data) != len(tiles) {
		tt.t.Errorf("SaveTiles: got %d data for %d tiles", len(data), len(tiles))
		return
	}
	for i, tile := range tiles {
		want, err := ReadTileData(tile, tt.hashes)
		if err != nil {
			tt.t.Errorf("SaveTiles(%v): %v", tile.Path(), err)
			continue
		}
		if !bytes.Equal(data[i], want) {
			tt.t.Errorf("SaveTiles(%v): saved data does not match tree", tile.Path())
		}
	}
}

// zeroIndex returns a TileReader that serves the same tiles as tr,
// except that the hash at the given stored index is replaced with an
// all-zeroes hash. If the index is not in the bottom row of its tile,
// the bottom-row hashes it is computed from are zeroed instead.
func zeroIndex(tr TileReader, index int64) TileReader {
	return &zeroIndexReader{tr, index}
}

type zeroIndexReader struct {
	TileReader
	index int64
}

func (r *zeroIndexReader) ReadTiles(tiles []Tile) ([][]byte, error) {
	data, err := r.TileReader.ReadTiles(tiles)
	if err != nil {
		return nil, err
	}
	t, start, end := tileForIndex(r.Height(), r.index)
	for i, tile := range tiles {
		if tile.H == t.H && tile.L == t.L && tile.N == t.N && end <= len(data[i]) {
			data[i] = bytes.Clone(data[i])
			clear(data[i][start:end])
		}
	}
	return data, nil
}

func TestTileHashReader(t *testing.T) {
	tt := newTestTree(t, 2)
	for range int64(100) {
		tt.Add(fmt.Appendf(nil, "leaf %d", tt.n))
		t.Run(fmt.Sprintf("N=%d", tt.n), func(t *testing.T) {
			th := TileHashReader(tt.Tree(), tt)
			for i := range StoredHashIndex(0, tt.n) {
				hashes, err := th.ReadHashes([]int64{i})
				if err != nil {
					t.Fatal(err)
				}
				if len(hashes) != 1 {
					t.Fatalf("ReadHashes(%d) = %d hashes, want 1", i, len(hashes))
				}
				if hashes[0] != tt.hashes[i] {
					t.Errorf("ReadHashes(%d) = %x, want %x", i, hashes[0], tt.hashes[i])
				}
			}

			var indexes []int64
			for j := range StoredHashIndex(0, tt.n) {
				indexes = append(indexes, j)
			}
			all, err := th.ReadHashes(indexes)
			if err != nil {
				t.Fatal(err)
			}
			if len(all) != len(tt.hashes) {
				t.Fatalf("ReadHashes(%d) = %d hashes, want %d", tt.n, len(all), len(tt.hashes))
			}
			for j, h := range all {
				if h != tt.hashes[j] {
					t.Errorf("ReadHashes(%d)[%d] = %v, want %v", tt.n, j, h, tt.hashes[j])
				}
			}

			for i := range StoredHashIndex(0, tt.n) {
				t.Run(fmt.Sprintf("tampered=%d", i), func(t *testing.T) {
					thz := TileHashReader(tt.Tree(), zeroIndex(tt, i))

					hashes, err := thz.ReadHashes([]int64{i})
					if err == nil {
						t.Errorf("ReadHashes(%d) = %v, want error", i, hashes[0])
					}

					all, err := thz.ReadHashes(indexes)
					if err == nil {
						t.Errorf("ReadHashes(%d) = %d hashes, want error", tt.n, len(all))
					}
				})
			}
		})
	}
}

// FuzzParseTilePath tests that ParseTilePath never crashes
func FuzzParseTilePath(f *testing.F) {
	f.Add("tile/4/0/001")
	f.Add("tile/4/0/001.p/5")
	f.Add("tile/3/5/x123/x456/078")
	f.Add("tile/3/5/x123/x456/078.p/2")
	f.Add("tile/1/0/x003/x057/500")
	f.Add("tile/3/5/123/456/078")
	f.Add("tile/3/-1/123/456/078")
	f.Add("tile/1/data/x003/x057/500")
	f.Fuzz(func(t *testing.T, path string) {
		ParseTilePath(path)
	})
}

func TestNewTilesForSize(t *testing.T) {
	for _, tt := range []struct {
		old, new int64
		want     int
	}{
		{1, 1, 0},
		{100, 101, 1},
		{1023, 1025, 3},
		{1024, 1030, 1},
		{1030, 2000, 1},
		{1030, 10000, 10},
		{49516517, 49516586, 3},
	} {
		t.Run(fmt.Sprintf("%d-%d", tt.old, tt.new), func(t *testing.T) {
			tiles := NewTiles(10, tt.old, tt.new)
			if got := len(tiles); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
				for _, tile := range tiles {
					t.Logf("%+v", tile)
				}
			}
		})
	}
}
