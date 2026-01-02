// Copyright (c) 2019-2020 The Zcash developers
// Copyright (c) 2025 Juno Cash developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or https://www.opensource.org/licenses/mit-license.php .

// Juno Cash: Orchard-only, no Sapling or Sprout support.
package parser

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// Some of these values may be "null" (which translates to nil in Go) in
// the test data, so we have *_set variables to indicate if the corresponding
// variable is non-null. (There is an "optional" package we could use for
// these but it doesn't seem worth pulling it in.)
type TxTestData struct {
	Tx                 string
	Txid               string
	Version            int
	NVersionGroupId    int
	NConsensusBranchId int
	Tx_in_count        int
	Tx_out_count       int
	NSpendsSapling     int
	NoutputsSapling    int
	NActionsOrchard    int
}

// https://jhall.io/posts/go-json-tricks-array-as-structs/
func (r *TxTestData) UnmarshalJSON(p []byte) error {
	var t []interface{}
	if err := json.Unmarshal(p, &t); err != nil {
		return err
	}
	r.Tx = t[0].(string)
	r.Txid = t[1].(string)
	r.Version = int(t[2].(float64))
	r.NVersionGroupId = int(t[3].(float64))
	r.NConsensusBranchId = int(t[4].(float64))
	r.Tx_in_count = int(t[7].(float64))
	r.Tx_out_count = int(t[8].(float64))
	r.NSpendsSapling = int(t[9].(float64))
	r.NoutputsSapling = int(t[10].(float64))
	r.NActionsOrchard = int(t[14].(float64))
	return nil
}

func TestV5TransactionParser(t *testing.T) {
	// The raw data are stored in a separate file because they're large enough
	// to make the test table difficult to scroll through. They are in the same
	// order as the test table above. If you update the test table without
	// adding a line to the raw file, this test will panic due to index
	// misalignment.
	s, err := os.ReadFile("../testdata/tx_v5.json")
	if err != nil {
		t.Fatal(err)
	}

	var testdata []json.RawMessage
	err = json.Unmarshal(s, &testdata)
	if err != nil {
		t.Fatal(err)
	}
	if len(testdata) < 3 {
		t.Fatal("tx_vt.json has too few lines")
	}
	testdata = testdata[2:]
	for _, onetx := range testdata {
		var txtestdata TxTestData

		err = json.Unmarshal(onetx, &txtestdata)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("txid %s", txtestdata.Txid)

		// Juno Cash: Skip transactions with Sapling elements
		if txtestdata.NSpendsSapling > 0 || txtestdata.NoutputsSapling > 0 {
			t.Logf("Skipping transaction with Sapling elements (not supported in Juno Cash)")
			continue
		}

		rawTxData, _ := hex.DecodeString(txtestdata.Tx)

		tx := NewTransaction()
		rest, err := tx.ParseFromSlice(rawTxData)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if len(rest) != 0 {
			t.Fatalf("Test did not consume entire buffer, %d remaining", len(rest))
		}
		// Currently, we can't check the txid because we get that from
		// zcashd (getblock rpc) rather than computing it ourselves.
		// https://github.com/zcash/lightwalletd/issues/392
		if tx.version != uint32(txtestdata.Version) {
			t.Fatal("version miscompare")
		}
		if tx.nVersionGroupID != uint32(txtestdata.NVersionGroupId) {
			t.Fatal("nVersionGroupId miscompare")
		}
		if tx.consensusBranchID != uint32(txtestdata.NConsensusBranchId) {
			t.Fatal("consensusBranchID miscompare")
		}
		if len(tx.transparentInputs) != int(txtestdata.Tx_in_count) {
			t.Fatal("tx_in_count miscompare")
		}
		if len(tx.transparentOutputs) != int(txtestdata.Tx_out_count) {
			t.Fatal("tx_out_count miscompare")
		}
		// Juno Cash: Sapling not supported, expect 0
		if tx.SaplingOutputsCount() != 0 {
			t.Fatal("Expected 0 Sapling outputs in Juno Cash")
		}
		if len(tx.orchardActions) != int(txtestdata.NActionsOrchard) {
			t.Fatal("NActionsOrchard miscompare")
		}
	}
}
