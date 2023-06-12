package smt

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
)

// SparseMerkleSumProof is a Merkle proof for an element in a SparseMerkleTree.
type SparseMerkleSumProof struct {
	// SideNodes is an array of the sibling nodes leading up to the leaf of the proof.
	SideNodes [][]byte

	// NonMembershipLeafData is the data of the unrelated leaf at the position
	// of the key being proven, in the case of a non-membership proof. For
	// membership proofs, is nil.
	NonMembershipLeafData []byte

	// SiblingData is the data of the sibling node to the leaf being proven,
	// required for updatable proofs. For unupdatable proofs, is nil.
	SiblingData []byte
}

func (proof *SparseMerkleSumProof) sanityCheck(spec *TreeSpec) bool {
	// Do a basic sanity check on the proof, so that a malicious proof cannot
	// cause the verifier to fatally exit (e.g. due to an index out-of-range
	// error) or cause a CPU DoS attack.

	// Check that the number of supplied sidenodes does not exceed the maximum possible.
	if len(proof.SideNodes) > spec.ph.PathSize()*8 ||

		// Check that leaf data for non-membership proofs is a valid size.
		(proof.NonMembershipLeafData != nil && len(proof.NonMembershipLeafData) < len(leafPrefix)+spec.ph.PathSize()) {
		return false
	}

	// Check that all supplied sidenodes are the correct size.
	for _, v := range proof.SideNodes {
		if len(v) != spec.th.hashSize()+16 {
			return false
		}
	}

	// Check that the sibling data hashes to the first side node if not nil
	if proof.SiblingData == nil || len(proof.SideNodes) == 0 {
		return true
	}

	siblingHash := hashSumSerialization(spec, proof.SiblingData)
	return bytes.Equal(proof.SideNodes[0], siblingHash)
}

// SparseCompactMerkleProof is a compact Merkle proof for an element in a SparseMerkleTree.
type SparseCompactMerkleSumProof struct {
	// SideNodes is an array of the sibling nodes leading up to the leaf of the proof.
	SideNodes [][]byte

	// NonMembershipLeafData is the data of the unrelated leaf at the position
	// of the key being proven, in the case of a non-membership proof. For
	// membership proofs, is nil.
	NonMembershipLeafData []byte

	// BitMask, in the case of a compact proof, is a bit mask of the sidenodes
	// of the proof where an on-bit indicates that the sidenode at the bit's
	// index is a placeholder. This is only set if the proof is compact.
	BitMask []byte

	// NumSideNodes, in the case of a compact proof, indicates the number of
	// sidenodes in the proof when decompacted. This is only set if the proof is compact.
	NumSideNodes int

	// SiblingData is the data of the sibling node to the leaf being proven,
	// required for updatable proofs. For unupdatable proofs, is nil.
	SiblingData []byte
}

func (proof *SparseCompactMerkleSumProof) sanityCheck(spec *TreeSpec) bool {
	// Do a basic sanity check on the proof on the fields of the proof specific to
	// the compact proof only.
	//
	// When the proof is de-compacted and verified, the sanity check for the
	// de-compacted proof should be executed.

	// Compact proofs: check that NumSideNodes is within the right range.
	if proof.NumSideNodes < 0 || proof.NumSideNodes > spec.ph.PathSize()*8 ||

		// Compact proofs: check that the length of the bit mask is as expected
		// according to NumSideNodes.
		len(proof.BitMask) != int(math.Ceil(float64(proof.NumSideNodes)/float64(8))) ||

		// Compact proofs: check that the correct number of sidenodes have been
		// supplied according to the bit mask.
		(proof.NumSideNodes > 0 && len(proof.SideNodes) != proof.NumSideNodes-countSetBits(proof.BitMask)) {
		return false
	}

	return true
}

// VerifyProof verifies a Merkle proof.
func VerifySumProof(proof SparseMerkleSumProof, root []byte, key []byte, value []byte, sum uint64, spec *TreeSpec) (bool, error) {
	var hexSum [16]byte
	hexBz, err := hex.DecodeString(fmt.Sprintf("%016x", sum))
	if err != nil {
		return false, err
	}
	copy(hexSum[16-len(hexBz):], hexBz)
	result, _, err := verifySumProofWithUpdates(proof, root, key, value, hexSum, spec)
	if err != nil {
		return false, err
	}
	return result, nil
}

func verifySumProofWithUpdates(proof SparseMerkleSumProof, root []byte, key []byte, value []byte, sum [16]byte, spec *TreeSpec) (bool, [][][]byte, error) {
	path := spec.ph.Path(key)

	if !proof.sanityCheck(spec) {
		return false, nil, ErrBadProof
	}

	var updates [][][]byte

	// Determine what the leaf hash should be.
	var currentHash, currentData []byte
	if bytes.Equal(value, defaultValue) { // Non-membership proof.
		if proof.NonMembershipLeafData == nil { // Leaf is a placeholder value.
			currentHash = spec.th.sumPlaceholder()
		} else { // Leaf is an unrelated leaf.
			actualPath, valueHash, actualSum := parseSumLeaf(proof.NonMembershipLeafData, spec.ph)
			if bytes.Equal(actualPath, path) {
				// This is not an unrelated leaf; non-membership proof failed.
				return false, nil, ErrBadProof
			}
			currentHash, currentData = spec.th.digestSumLeaf(actualPath, valueHash, actualSum)

			update := make([][]byte, 2)
			update[0], update[1] = currentHash, currentData
			updates = append(updates, update)
		}
	} else { // Membership proof.
		valueHash := spec.digestValue(value)
		currentHash, currentData = spec.th.digestSumLeaf(path, valueHash, sum)
		update := make([][]byte, 2)
		update[0], update[1] = currentHash, currentData
		updates = append(updates, update)
	}

	// Recompute root.
	var err error
	for i := 0; i < len(proof.SideNodes); i++ {
		node := make([]byte, spec.th.hashSize()+16)
		copy(node, proof.SideNodes[i])

		if getPathBit(path, len(proof.SideNodes)-1-i) == left {
			currentHash, currentData, err = spec.th.digestSumNode(currentHash, node)
			if err != nil {
				return false, nil, err
			}
		} else {
			currentHash, currentData, err = spec.th.digestSumNode(node, currentHash)
			if err != nil {
				return false, nil, err
			}
		}

		update := make([][]byte, 2)
		update[0], update[1] = currentHash, currentData
		updates = append(updates, update)
	}

	return bytes.Equal(currentHash, root), updates, nil
}

// VerifyCompactProof verifies a compacted Merkle proof.
func VerifyCompactSumProof(proof SparseCompactMerkleSumProof, root []byte, key, value []byte, sum uint64, spec *TreeSpec) (bool, error) {
	decompactedProof, err := DecompactSumProof(proof, spec)
	if err != nil {
		return false, err
	}
	return VerifySumProof(decompactedProof, root, key, value, sum, spec)
}

// CompactProof compacts a proof, to reduce its size.
func CompactSumProof(proof SparseMerkleSumProof, spec *TreeSpec) (SparseCompactMerkleSumProof, error) {
	if !proof.sanityCheck(spec) {
		return SparseCompactMerkleSumProof{}, ErrBadProof
	}

	bitMask := make([]byte, int(math.Ceil(float64(len(proof.SideNodes))/float64(8))))
	var compactedSideNodes [][]byte
	for i := 0; i < len(proof.SideNodes); i++ {
		node := make([]byte, spec.th.hashSize()+16)
		copy(node, proof.SideNodes[i])
		if bytes.Equal(node, spec.th.sumPlaceholder()) {
			setPathBit(bitMask, i)
		} else {
			compactedSideNodes = append(compactedSideNodes, node)
		}
	}

	return SparseCompactMerkleSumProof{
		SideNodes:             compactedSideNodes,
		NonMembershipLeafData: proof.NonMembershipLeafData,
		BitMask:               bitMask,
		NumSideNodes:          len(proof.SideNodes),
		SiblingData:           proof.SiblingData,
	}, nil
}

// DecompactProof decompacts a proof, so that it can be used for VerifyProof.
func DecompactSumProof(proof SparseCompactMerkleSumProof, spec *TreeSpec) (SparseMerkleSumProof, error) {
	if !proof.sanityCheck(spec) {
		return SparseMerkleSumProof{}, ErrBadProof
	}

	decompactedSideNodes := make([][]byte, proof.NumSideNodes)
	position := 0
	for i := 0; i < proof.NumSideNodes; i++ {
		if getPathBit(proof.BitMask, i) == 1 {
			decompactedSideNodes[i] = spec.th.sumPlaceholder()
		} else {
			decompactedSideNodes[i] = proof.SideNodes[position]
			position++
		}
	}

	return SparseMerkleSumProof{
		SideNodes:             decompactedSideNodes,
		NonMembershipLeafData: proof.NonMembershipLeafData,
		SiblingData:           proof.SiblingData,
	}, nil
}
