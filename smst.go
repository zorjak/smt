package smt

import (
	"bytes"
	"encoding/binary"
	"hash"
)

var _ SparseMerkleSumTree = (*SMST)(nil)

// Sparse Merkle Sum Tree object wrapping a Sparse Merkle Tree for custom encoding
type SMST struct {
	TreeSpec
	*SMT
}

// NewSparseMerkleSumTree returns a pointer to an SMST struct
func NewSparseMerkleSumTree(nodes MapStore, hasher hash.Hash, options ...Option) *SMST {
	smt := &SMT{
		TreeSpec: newTreeSpec(hasher, true),
		nodes:    nodes,
	}
	for _, option := range options {
		option(&smt.TreeSpec)
	}
	nvh := WithValueHasher(nil)
	nvh(&smt.TreeSpec)
	smst := &SMST{
		TreeSpec: newTreeSpec(hasher, true),
		SMT:      smt,
	}
	for _, option := range options {
		option(&smst.TreeSpec)
	}
	return smst
}

// ImportSparseMerkleSumTree returns a pointer to an SMST struct with the root hash provided
func ImportSparseMerkleSumTree(nodes MapStore, hasher hash.Hash, root []byte, options ...Option) *SMST {
	smst := NewSparseMerkleSumTree(nodes, hasher, options...)
	smst.tree = &lazyNode{root}
	smst.savedRoot = root
	return smst
}

// Spec returns the SMST TreeSpec
func (smst *SMST) Spec() *TreeSpec {
	return &smst.TreeSpec
}

// Get returns the digest of the value stored at the given key and the sum of the leaf node
func (smst *SMST) Get(key []byte) ([]byte, uint64, error) {
	valueHash, err := smst.SMT.Get(key)
	if err != nil {
		return nil, 0, err
	}
	if bytes.Equal(valueHash, defaultValue) {
		return defaultValue, 0, nil
	}
	var sumBz [sumSize]byte
	copy(sumBz[:], valueHash[len(valueHash)-sumSize:])
	sum := binary.BigEndian.Uint64(sumBz[:])
	return valueHash[:len(valueHash)-sumSize], sum, nil
}

// Update sets the value for the given key, to the digest of the provided value
func (smst *SMST) Update(key []byte, value []byte, sum uint64) error {
	valueHash := smst.digestValue(value)
	var sumBz [sumSize]byte
	binary.BigEndian.PutUint64(sumBz[:], sum)
	valueHash = append(valueHash, sumBz[:]...)
	return smst.SMT.Update(key, valueHash)
}

// Delete removes the node at the path corresponding to the given key
func (smst *SMST) Delete(key []byte) error {
	return smst.SMT.Delete(key)
}

// Prove generates a SparseMerkleProof for the given key
func (smst *SMST) Prove(key []byte) (proof SparseMerkleProof, err error) {
	return smst.SMT.Prove(key)
}

// Commit persists all dirty nodes in the tree, deletes all orphaned
// nodes from the database and then computes and saves the root hash
func (smst *SMST) Commit() (err error) {
	return smst.SMT.Commit()
}

func (smst *SMST) Root() []byte {
	return smst.SMT.Root() // [digest]+[sumSize byte hex sum]
}

// Sum returns the uint64 sum of the entire tree
func (smst *SMST) Sum() uint64 {
	var sumBz [sumSize]byte
	digest := smst.Root()
	copy(sumBz[:], digest[len(digest)-sumSize:])
	return binary.BigEndian.Uint64(sumBz[:])
}
