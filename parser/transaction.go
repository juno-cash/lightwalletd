// Copyright (c) 2019-2020 The Zcash developers
// Copyright (c) 2025 Juno Cash developers
// Distributed under the MIT software license, see the accompanying
// file COPYING or https://www.opensource.org/licenses/mit-license.php .

// Package parser deserializes (full) transactions.
// Juno Cash: Orchard-only, no Sapling or Sprout support.
package parser

import (
	"errors"
	"fmt"

	"github.com/zcash/lightwalletd/hash32"
	"github.com/zcash/lightwalletd/parser/internal/bytestring"
	"github.com/zcash/lightwalletd/walletrpc"
)

type rawTransaction struct {
	fOverwintered      bool
	version            uint32
	nVersionGroupID    uint32
	consensusBranchID  uint32
	transparentInputs  []txIn
	transparentOutputs []txOut
	// Juno Cash: Orchard-only, no Sapling or Sprout support
	orchardActions []action
}

// Txin format as described in https://en.bitcoin.it/wiki/Transaction
type txIn struct {
	// SHA256d of a previous (to-be-used) transaction
	//PrevTxHash []byte

	// Index of the to-be-used output in the previous tx
	//PrevTxOutIndex uint32

	// CompactSize-prefixed, could be a pubkey or a script
	ScriptSig []byte

	// Bitcoin: "normally 0xFFFFFFFF; irrelevant unless transaction's lock_time > 0"
	//SequenceNumber uint32
}

func (tx *txIn) ParseFromSlice(data []byte) ([]byte, error) {
	s := bytestring.String(data)

	if !s.Skip(32) {
		return nil, errors.New("could not skip PrevTxHash")
	}

	if !s.Skip(4) {
		return nil, errors.New("could not skip PrevTxOutIndex")
	}

	if !s.ReadCompactLengthPrefixed((*bytestring.String)(&tx.ScriptSig)) {
		return nil, errors.New("could not read ScriptSig")
	}

	if !s.Skip(4) {
		return nil, errors.New("could not skip SequenceNumber")
	}

	return []byte(s), nil
}

// Txout format as described in https://en.bitcoin.it/wiki/Transaction
type txOut struct {
	// Non-negative int giving the number of zatoshis to be transferred
	Value uint64

	// Script. CompactSize-prefixed.
	//Script []byte
}

func (tx *txOut) ParseFromSlice(data []byte) ([]byte, error) {
	s := bytestring.String(data)

	if !s.Skip(8) {
		return nil, errors.New("could not skip txOut value")
	}

	if !s.SkipCompactLengthPrefixed() {
		return nil, errors.New("could not skip txOut script")
	}

	return []byte(s), nil
}

// parse the transparent parts of the transaction
func (tx *Transaction) ParseTransparent(data []byte) ([]byte, error) {
	s := bytestring.String(data)
	var txInCount int
	if !s.ReadCompactSize(&txInCount) {
		return nil, errors.New("could not read tx_in_count")
	}
	var err error
	tx.transparentInputs = make([]txIn, txInCount)
	for i := 0; i < txInCount; i++ {
		ti := &tx.transparentInputs[i]
		s, err = ti.ParseFromSlice([]byte(s))
		if err != nil {
			return nil, fmt.Errorf("error parsing transparent input: %w", err)
		}
	}

	var txOutCount int
	if !s.ReadCompactSize(&txOutCount) {
		return nil, errors.New("could not read tx_out_count")
	}
	tx.transparentOutputs = make([]txOut, txOutCount)
	for i := 0; i < txOutCount; i++ {
		to := &tx.transparentOutputs[i]
		s, err = to.ParseFromSlice([]byte(s))
		if err != nil {
			return nil, fmt.Errorf("error parsing transparent output: %w", err)
		}
	}
	return []byte(s), nil
}

// Juno Cash: Sapling spend/output and JoinSplit types removed (Orchard-only)

type action struct {
	//cv            []byte // 32
	nullifier []byte // 32
	//rk            []byte // 32
	cmx           []byte // 32
	ephemeralKey  []byte // 32
	encCiphertext []byte // 580
	//outCiphertext []byte // 80
}

func (a *action) ParseFromSlice(data []byte) ([]byte, error) {
	s := bytestring.String(data)
	if !s.Skip(32) {
		return nil, errors.New("could not read action cv")
	}
	if !s.ReadBytes(&a.nullifier, 32) {
		return nil, errors.New("could not read action nullifier")
	}
	if !s.Skip(32) {
		return nil, errors.New("could not read action rk")
	}
	if !s.ReadBytes(&a.cmx, 32) {
		return nil, errors.New("could not read action cmx")
	}
	if !s.ReadBytes(&a.ephemeralKey, 32) {
		return nil, errors.New("could not read action ephemeralKey")
	}
	if !s.ReadBytes(&a.encCiphertext, 580) {
		return nil, errors.New("could not read action encCiphertext")
	}
	if !s.Skip(80) {
		return nil, errors.New("could not read action outCiphertext")
	}
	return []byte(s), nil
}

func (p *action) ToCompact() *walletrpc.CompactOrchardAction {
	return &walletrpc.CompactOrchardAction{
		Nullifier:    p.nullifier,
		Cmx:          p.cmx,
		EphemeralKey: p.ephemeralKey,
		Ciphertext:   p.encCiphertext[:52],
	}
}

// Transaction encodes a full (zcashd) transaction.
type Transaction struct {
	*rawTransaction
	rawBytes []byte
	txID     hash32.T // from getblock verbose=1
}

func (tx *Transaction) SetTxID(txid hash32.T) {
	tx.txID = txid
}

// GetDisplayHashSring returns the transaction hash in hex big-endian display order.
func (tx *Transaction) GetDisplayHashString() string {
	return hash32.Encode(hash32.Reverse(tx.txID))
}

// GetEncodableHash returns the transaction hash in little-endian wire format order.
func (tx *Transaction) GetEncodableHash() hash32.T {
	return tx.txID
}

// Bytes returns a full transaction's raw bytes.
func (tx *Transaction) Bytes() []byte {
	return tx.rawBytes
}

// HasShieldedElements indicates whether a transaction has
// at least one shielded (Orchard) input or output.
// Juno Cash: Only Orchard is supported.
func (tx *Transaction) HasShieldedElements() bool {
	return tx.version >= 5 && len(tx.orchardActions) > 0
}

// SaplingOutputsCount returns the number of Sapling outputs in the transaction.
// Juno Cash: Always returns 0 (Sapling not supported).
func (tx *Transaction) SaplingOutputsCount() int {
	return 0
}

// OrchardActionsCount returns the number of Orchard actions in the transaction.
func (tx *Transaction) OrchardActionsCount() int {
	return len(tx.orchardActions)
}

// ToCompact converts the given (full) transaction to compact format.
// Juno Cash: Only Orchard actions are populated (no Sapling).
func (tx *Transaction) ToCompact(index int) *walletrpc.CompactTx {
	ctx := &walletrpc.CompactTx{
		Index:   uint64(index), // index is contextual
		Txid:    hash32.ToSlice(tx.GetEncodableHash()),
		Actions: make([]*walletrpc.CompactOrchardAction, len(tx.orchardActions)),
		// Juno Cash: Spends and Outputs (Sapling) are always empty
	}
	for i, a := range tx.orchardActions {
		ctx.Actions[i] = a.ToCompact()
	}
	return ctx
}

// parse version 4 transaction data after the nVersionGroupId field.
// Juno Cash: V4 transactions are only allowed for coinbase (transparent-only).
// Sapling and JoinSplit data must be empty.
func (tx *Transaction) parseV4(data []byte) ([]byte, error) {
	s := bytestring.String(data)
	var err error
	if tx.nVersionGroupID != 0x892F2085 {
		return nil, fmt.Errorf("version group ID %x must be 0x892F2085", tx.nVersionGroupID)
	}
	s, err = tx.ParseTransparent([]byte(s))
	if err != nil {
		return nil, err
	}
	if !s.Skip(4) {
		return nil, errors.New("could not skip nLockTime")
	}

	if !s.Skip(4) {
		return nil, errors.New("could not skip nExpiryHeight")
	}

	var spendCount, outputCount int

	if !s.Skip(8) {
		return nil, errors.New("could not skip valueBalance")
	}
	if !s.ReadCompactSize(&spendCount) {
		return nil, errors.New("could not read nShieldedSpend")
	}
	// Juno Cash: Sapling spends not allowed
	if spendCount > 0 {
		return nil, errors.New("Juno Cash: Sapling spends not supported")
	}
	if !s.ReadCompactSize(&outputCount) {
		return nil, errors.New("could not read nShieldedOutput")
	}
	// Juno Cash: Sapling outputs not allowed
	if outputCount > 0 {
		return nil, errors.New("Juno Cash: Sapling outputs not supported")
	}
	var joinSplitCount int
	if !s.ReadCompactSize(&joinSplitCount) {
		return nil, errors.New("could not read nJoinSplit")
	}
	// Juno Cash: JoinSplits (Sprout) not allowed
	if joinSplitCount > 0 {
		return nil, errors.New("Juno Cash: JoinSplits (Sprout) not supported")
	}
	return s, nil
}

// parse version 5 transaction data after the nVersionGroupId field.
// Juno Cash: Only Orchard is supported. Sapling data must be empty.
func (tx *Transaction) parseV5(data []byte) ([]byte, error) {
	s := bytestring.String(data)
	var err error
	if !s.ReadUint32(&tx.consensusBranchID) {
		return nil, errors.New("could not read nVersionGroupId")
	}
	if tx.nVersionGroupID != 0x26A7270A {
		return nil, errors.New(fmt.Sprintf("version group ID %d must be 0x26A7270A", tx.nVersionGroupID))
	}
	if !s.Skip(4) {
		return nil, errors.New("could not skip nLockTime")
	}
	if !s.Skip(4) {
		return nil, errors.New("could not skip nExpiryHeight")
	}
	s, err = tx.ParseTransparent([]byte(s))
	if err != nil {
		return nil, err
	}

	var spendCount, outputCount int
	if !s.ReadCompactSize(&spendCount) {
		return nil, errors.New("could not read nShieldedSpend")
	}
	// Juno Cash: Sapling spends not allowed
	if spendCount > 0 {
		return nil, errors.New("Juno Cash: Sapling spends not supported")
	}
	if !s.ReadCompactSize(&outputCount) {
		return nil, errors.New("could not read nShieldedOutput")
	}
	// Juno Cash: Sapling outputs not allowed
	if outputCount > 0 {
		return nil, errors.New("Juno Cash: Sapling outputs not supported")
	}

	// Parse Orchard actions
	var actionsCount int
	if !s.ReadCompactSize(&actionsCount) {
		return nil, errors.New("could not read nActionsOrchard")
	}
	if actionsCount >= (1 << 16) {
		return nil, errors.New(fmt.Sprintf("actionsCount (%d) must be less than 2^16", actionsCount))
	}
	tx.orchardActions = make([]action, actionsCount)
	for i := 0; i < actionsCount; i++ {
		a := &tx.orchardActions[i]
		s, err = a.ParseFromSlice([]byte(s))
		if err != nil {
			return nil, fmt.Errorf("error parsing orchard action: %w", err)
		}
	}
	if actionsCount > 0 {
		if !s.Skip(1) {
			return nil, errors.New("could not skip flagsOrchard")
		}
		if !s.Skip(8) {
			return nil, errors.New("could not skip valueBalanceOrchard")
		}
		if !s.Skip(32) {
			return nil, errors.New("could not skip anchorOrchard")
		}
		var proofsCount int
		if !s.ReadCompactSize(&proofsCount) {
			return nil, errors.New("could not read sizeProofsOrchard")
		}
		if !s.Skip(proofsCount) {
			return nil, errors.New("could not skip proofsOrchard")
		}
		if !s.Skip(64 * actionsCount) {
			return nil, errors.New("could not skip vSpendAuthSigsOrchard")
		}
		if !s.Skip(64) {
			return nil, errors.New("could not skip bindingSigOrchard")
		}
	}
	return s, nil
}

// ParseFromSlice deserializes a single transaction from the given data.
func (tx *Transaction) ParseFromSlice(data []byte) ([]byte, error) {
	s := bytestring.String(data)

	// declare here to prevent shadowing problems in cryptobyte assignments
	var err error

	var header uint32
	if !s.ReadUint32(&header) {
		return nil, errors.New("could not read header")
	}

	tx.fOverwintered = (header >> 31) == 1
	if !tx.fOverwintered {
		return nil, errors.New("fOverwinter flag must be set")
	}
	tx.version = header & 0x7FFFFFFF
	if tx.version < 4 {
		return nil, errors.New(fmt.Sprintf("version number %d must be greater or equal to 4", tx.version))
	}

	if !s.ReadUint32(&tx.nVersionGroupID) {
		return nil, errors.New("could not read nVersionGroupId")
	}
	// parse the main part of the transaction
	if tx.version <= 4 {
		s, err = tx.parseV4([]byte(s))
	} else {
		s, err = tx.parseV5([]byte(s))
	}
	if err != nil {
		return nil, err
	}
	// TODO: implement rawBytes with MarshalBinary() instead
	txLen := len(data) - len(s)
	tx.rawBytes = data[:txLen]

	return []byte(s), nil
}

// NewTransaction is the constructor for a full transaction.
func NewTransaction() *Transaction {
	return &Transaction{
		rawTransaction: new(rawTransaction),
	}
}
