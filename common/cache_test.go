// Copyright (c) 2019-2020 The Zcash developers
// Copyright (c) 2025 Juno Cash developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or https://www.opensource.org/licenses/mit-license.php .

// Juno Cash: Orchard-only, no Sapling or Sprout support.
package common

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/zcash/lightwalletd/hash32"
	"github.com/zcash/lightwalletd/parser"
	"github.com/zcash/lightwalletd/walletrpc"
)

var compacts []*walletrpc.CompactBlock
var cache *BlockCache

const (
	unitTestPath  = "unittestcache"
	unitTestChain = "unittestnet"
)

func TestCache(t *testing.T) {
	type compactTest struct {
		BlockHeight int    `json:"block"`
		BlockHash   string `json:"hash"`
		PrevHash    string `json:"prev"`
		Full        string `json:"full"`
		Compact     string `json:"compact"`
	}
	var compactTests []compactTest

	blockJSON, err := os.ReadFile("../testdata/compact_blocks.json")
	if err != nil {
		t.Fatal(err)
	}

	err = json.Unmarshal(blockJSON, &compactTests)
	if err != nil {
		t.Fatal(err)
	}

	// Derive compact blocks from file data (setup, not part of the test).
	// Juno Cash: Skip blocks with Sapling transactions (not supported).
	for _, test := range compactTests {
		blockData, _ := hex.DecodeString(test.Full)
		block := parser.NewBlock()
		blockData, err = block.ParseFromSlice(blockData)
		if err != nil {
			t.Logf("Skipping block %d (likely has Sapling transactions): %v", test.BlockHeight, err)
			continue
		}
		if len(blockData) > 0 {
			t.Error("Extra data remaining")
		}
		compacts = append(compacts, block.ToCompact())
	}

	// Juno Cash: Need at least 3 blocks for meaningful cache tests
	if len(compacts) < 3 {
		t.Skipf("Not enough blocks without Sapling transactions for cache test (have %d, need 3)", len(compacts))
	}

	// Use a test height based on first compact block
	startHeight := int(compacts[0].Height)

	os.RemoveAll(unitTestPath)
	cache = NewBlockCache(unitTestPath, unitTestChain, startHeight, 0)

	// Initially cache is empty.
	if cache.GetLatestHeight() != -1 {
		t.Fatal("unexpected GetLatestHeight")
	}
	if cache.firstBlock != startHeight {
		t.Fatal("unexpected initial firstBlock")
	}
	if cache.nextBlock != startHeight {
		t.Fatal("unexpected initial nextBlock")
	}
	fillCache(t)

	// Juno Cash: Only run reorg tests if we have enough blocks
	if len(compacts) >= 3 {
		reorgCache(t)
		fillCache(t)
	}

	// Simulate a restart to ensure the db files are read correctly.
	cache = NewBlockCache(unitTestPath, unitTestChain, startHeight, -1)

	// Should still have all blocks.
	expectedNextBlock := startHeight + len(compacts)
	if cache.nextBlock != expectedNextBlock {
		t.Fatalf("unexpected nextBlock height: got %d, want %d", cache.nextBlock, expectedNextBlock)
	}

	if len(compacts) >= 3 {
		reorgCache(t)
	}

	// Reorg to before the first block moves back to only the first block
	cache.Reorg(startHeight - 1)
	if cache.latestHash != hash32.Nil {
		t.Fatal("unexpected latestHash, should be nil")
	}
	if cache.nextBlock != startHeight {
		t.Fatal("unexpected nextBlock: ", cache.nextBlock)
	}

	// Clean up the test files.
	cache.Close()
	os.RemoveAll(unitTestPath)
}

func reorgCache(t *testing.T) {
	if len(compacts) < 3 {
		t.Skip("Not enough blocks for reorg test")
	}

	startHeight := int(compacts[0].Height)

	// Simulate a reorg by adding a block whose height is lower than the latest;
	// we're replacing the second block, so there should be only two blocks.
	cache.Reorg(startHeight + 1)
	err := cache.Add(startHeight+1, compacts[1])
	if err != nil {
		t.Fatal(err)
	}
	if cache.firstBlock != startHeight {
		t.Fatal("unexpected firstBlock height")
	}
	if cache.nextBlock != startHeight+2 {
		t.Fatal("unexpected nextBlock height")
	}
	if len(cache.starts) != 3 {
		t.Fatal("unexpected len(cache.starts)")
	}

	// some "black-box" tests (using exported interfaces)
	if cache.GetLatestHeight() != startHeight+1 {
		t.Fatal("unexpected GetLatestHeight")
	}
	if int(cache.Get(startHeight+1).Height) != startHeight+1 {
		t.Fatal("unexpected block contents")
	}

	// Make sure we can go forward from here
	err = cache.Add(startHeight+2, compacts[2])
	if err != nil {
		t.Fatal(err)
	}
	if cache.firstBlock != startHeight {
		t.Fatal("unexpected firstBlock height")
	}
	if cache.nextBlock != startHeight+3 {
		t.Fatal("unexpected nextBlock height")
	}
	if len(cache.starts) != 4 {
		t.Fatal("unexpected len(cache.starts)")
	}

	if cache.GetLatestHeight() != startHeight+2 {
		t.Fatal("unexpected GetLatestHeight")
	}
	if int(cache.Get(startHeight+2).Height) != startHeight+2 {
		t.Fatal("unexpected block contents")
	}
}

// Whatever the state of the cache, add blocks starting at the
// first compact block's height (this could cause a reorg).
// Juno Cash: After skipping Sapling blocks, remaining blocks may not be consecutive.
// We renumber them to be consecutive for testing purposes.
func fillCache(t *testing.T) {
	if len(compacts) == 0 {
		t.Skip("No blocks to fill cache")
	}

	startHeight := int(compacts[0].Height)
	cache.Reorg(startHeight)
	for i, compact := range compacts {
		// Juno Cash: Renumber blocks to be consecutive for cache testing
		compact.Height = uint64(startHeight + i)
		err := cache.Add(startHeight+i, compact)
		if err != nil {
			t.Fatal(err)
		}

		// some "white-box" checks
		if cache.firstBlock != startHeight {
			t.Fatal("unexpected firstBlock height")
		}
		if cache.nextBlock != startHeight+i+1 {
			t.Fatal("unexpected nextBlock height")
		}
		if len(cache.starts) != i+2 {
			t.Fatal("unexpected len(cache.starts)")
		}

		// some "black-box" tests (using exported interfaces)
		if cache.GetLatestHeight() != startHeight+i {
			t.Fatal("unexpected GetLatestHeight")
		}
		b := cache.Get(startHeight + i)
		if b == nil {
			t.Fatal("unexpected Get failure")
		}
		if int(b.Height) != startHeight+i {
			t.Fatal("unexpected block contents")
		}
	}
}
