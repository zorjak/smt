package smt

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test base case Merkle proof operations.
func TestSMST_ProofsBasic(t *testing.T) {
	var smn, smv KVStore
	var smst *SMSTWithStorage
	var proof *SparseMerkleProof
	var result bool
	var root []byte
	var err error

	smn, err = NewKVStore("")
	require.NoError(t, err)
	smv, err = NewKVStore("")
	require.NoError(t, err)
	smst = NewSMSTWithStorage(smn, smv, sha256.New())
	base := smst.Spec()

	// Generate and verify a proof on an empty key.
	proof, err = smst.Prove([]byte("testKey3"))
	require.NoError(t, err)
	checkCompactEquivalence(t, proof, base)
	result = VerifySumProof(proof, placeholder(base), []byte("testKey3"), defaultValue, 0, base)
	require.True(t, result)
	result = VerifySumProof(proof, root, []byte("testKey3"), []byte("badValue"), 5, base)
	require.False(t, result)

	// Add a key, generate and verify a Merkle proof.
	err = smst.Update([]byte("testKey"), []byte("testValue"), 5)
	require.NoError(t, err)
	root = smst.Root()
	proof, err = smst.Prove([]byte("testKey"))
	require.NoError(t, err)
	checkCompactEquivalence(t, proof, base)
	result = VerifySumProof(proof, root, []byte("testKey"), []byte("testValue"), 5, base) // valid
	require.True(t, result)
	result = VerifySumProof(proof, root, []byte("testKey"), []byte("badValue"), 5, base) // wrong value
	require.False(t, result)
	result = VerifySumProof(proof, root, []byte("testKey"), []byte("testValue"), 10, base) // wrong sum
	require.False(t, result)
	result = VerifySumProof(proof, root, []byte("testKey"), []byte("badValue"), 10, base) // wrong value and sum
	require.False(t, result)

	// Add a key, generate and verify both Merkle proofs.
	err = smst.Update([]byte("testKey2"), []byte("testValue"), 5)
	require.NoError(t, err)
	root = smst.Root()
	proof, err = smst.Prove([]byte("testKey"))
	require.NoError(t, err)
	checkCompactEquivalence(t, proof, base)
	result = VerifySumProof(proof, root, []byte("testKey"), []byte("testValue"), 5, base) // valid
	require.True(t, result)
	result = VerifySumProof(proof, root, []byte("testKey"), []byte("badValue"), 5, base) // wrong value
	require.False(t, result)
	result = VerifySumProof(proof, root, []byte("testKey"), []byte("testValue"), 10, base) // wrong sum
	require.False(t, result)
	result = VerifySumProof(proof, root, []byte("testKey"), []byte("badValue"), 10, base) // wrong value and sum
	require.False(t, result)
	result = VerifySumProof(randomiseSumProof(proof), root, []byte("testKey"), []byte("testValue"), 5, base) // invalid proof
	require.False(t, result)

	proof, err = smst.Prove([]byte("testKey2"))
	require.NoError(t, err)
	checkCompactEquivalence(t, proof, base)
	result = VerifySumProof(proof, root, []byte("testKey2"), []byte("testValue"), 5, base) // valid
	require.True(t, result)
	result = VerifySumProof(proof, root, []byte("testKey2"), []byte("badValue"), 5, base) // wrong value
	require.False(t, result)
	result = VerifySumProof(proof, root, []byte("testKey2"), []byte("testValue"), 10, base) // wrong sum
	require.False(t, result)
	result = VerifySumProof(proof, root, []byte("testKey2"), []byte("badValue"), 10, base) // wrong value and sum
	require.False(t, result)
	result = VerifySumProof(randomiseSumProof(proof), root, []byte("testKey2"), []byte("testValue"), 5, base) // invalid proof
	require.False(t, result)

	// Try proving a default value for a non-default leaf.
	var sum [sumSize]byte
	binary.BigEndian.PutUint64(sum[:], 5)
	tval := base.digestValue([]byte("testValue"))
	tval = append(tval, sum[:]...)
	_, leafData := base.th.digestSumLeaf(base.ph.Path([]byte("testKey2")), tval)
	proof = &SparseMerkleProof{
		SideNodes:             proof.SideNodes,
		NonMembershipLeafData: leafData,
	}
	result = VerifySumProof(proof, root, []byte("testKey2"), defaultValue, 0, base)
	require.False(t, result)

	// Generate and verify a proof on an empty key.
	proof, err = smst.Prove([]byte("testKey3"))
	require.NoError(t, err)
	checkCompactEquivalence(t, proof, base)
	result = VerifySumProof(proof, root, []byte("testKey3"), defaultValue, 0, base) // valid
	require.True(t, result)
	result = VerifySumProof(proof, root, []byte("testKey3"), []byte("badValue"), 0, base) // wrong value
	require.False(t, result)
	result = VerifySumProof(proof, root, []byte("testKey3"), defaultValue, 5, base) // wrong sum
	require.False(t, result)
	result = VerifySumProof(randomiseSumProof(proof), root, []byte("testKey3"), defaultValue, 0, base) // invalid proof
	require.False(t, result)

	require.NoError(t, smn.Stop())
	require.NoError(t, smv.Stop())
}

// Test sanity check cases for non-compact proofs.
func TestSMST_ProofsSanityCheck(t *testing.T) {
	smn, err := NewKVStore("")
	require.NoError(t, err)
	smv, err := NewKVStore("")
	require.NoError(t, err)
	smst := NewSMSTWithStorage(smn, smv, sha256.New())
	base := smst.Spec()

	err = smst.Update([]byte("testKey1"), []byte("testValue1"), 1)
	require.NoError(t, err)
	err = smst.Update([]byte("testKey2"), []byte("testValue2"), 2)
	require.NoError(t, err)
	err = smst.Update([]byte("testKey3"), []byte("testValue3"), 3)
	require.NoError(t, err)
	err = smst.Update([]byte("testKey4"), []byte("testValue4"), 4)
	require.NoError(t, err)
	root := smst.Root()

	// Case: invalid number of sidenodes.
	proof, _ := smst.Prove([]byte("testKey1"))
	sideNodes := make([][]byte, smst.Spec().depth()+1)
	for i := range sideNodes {
		sideNodes[i] = proof.SideNodes[0]
	}
	proof.SideNodes = sideNodes
	require.False(t, proof.sanityCheck(base))
	result := VerifySumProof(proof, root, []byte("testKey1"), []byte("testValue1"), 1, base)
	require.False(t, result)
	_, err = CompactProof(proof, base)
	require.Error(t, err)

	// Case: incorrect size for NonMembershipLeafData.
	proof, _ = smst.Prove([]byte("testKey1"))
	proof.NonMembershipLeafData = make([]byte, 1)
	require.False(t, proof.sanityCheck(base))
	result = VerifySumProof(proof, root, []byte("testKey1"), []byte("testValue1"), 1, base)
	require.False(t, result)
	_, err = CompactProof(proof, base)
	require.Error(t, err)

	// Case: unexpected sidenode size.
	proof, _ = smst.Prove([]byte("testKey1"))
	proof.SideNodes[0] = make([]byte, 1)
	require.False(t, proof.sanityCheck(base))
	result = VerifySumProof(proof, root, []byte("testKey1"), []byte("testValue1"), 1, base)
	require.False(t, result)
	_, err = CompactProof(proof, base)
	require.Error(t, err)

	// Case: incorrect non-nil sibling data
	proof, _ = smst.Prove([]byte("testKey1"))
	proof.SiblingData = base.th.digest(proof.SiblingData)
	require.False(t, proof.sanityCheck(base))

	result = VerifySumProof(proof, root, []byte("testKey1"), []byte("testValue1"), 1, base)
	require.False(t, result)
	_, err = CompactProof(proof, base)
	require.Error(t, err)

	require.NoError(t, smn.Stop())
	require.NoError(t, smv.Stop())
}

// ProveClosest test against a visual representation of the tree
// See: https://github.com/pokt-network/smt/assets/53987565/2a2f33e0-f81f-41c5-bd76-af0cd1cd8f15
func TestSMST_ProveClosest(t *testing.T) {
	var smn KVStore
	var smst *SMST
	var proof *SparseMerkleProof
	var result bool
	var root, closestKey, closestValueHash []byte
	var closestSum uint64
	var err error

	smn, err = NewKVStore("")
	require.NoError(t, err)
	smst = NewSparseMerkleSumTree(smn, sha256.New(), WithValueHasher(nil))

	// insert some unrelated values to populate the tree
	require.NoError(t, smst.Update([]byte("foo"), []byte("oof"), 3))
	require.NoError(t, smst.Update([]byte("bar"), []byte("rab"), 6))
	require.NoError(t, smst.Update([]byte("baz"), []byte("zab"), 9))
	require.NoError(t, smst.Update([]byte("bin"), []byte("nib"), 12))
	require.NoError(t, smst.Update([]byte("fiz"), []byte("zif"), 15))
	require.NoError(t, smst.Update([]byte("fob"), []byte("bof"), 18))
	require.NoError(t, smst.Update([]byte("testKey"), []byte("testValue"), 21))
	require.NoError(t, smst.Update([]byte("testKey2"), []byte("testValue2"), 24))
	require.NoError(t, smst.Update([]byte("testKey3"), []byte("testValue3"), 27))
	require.NoError(t, smst.Update([]byte("testKey4"), []byte("testValue4"), 30))

	root = smst.Root()

	// `testKey2` is the child of an inner node, which is the child of an extension node.
	// The extension node has the path bounds of [3, 7]. This means any bits between
	// 3-6 can be flipped, and the resulting path would still traverse through the same
	// extension node and lead to testKey2 - the closest key. However, flipping bit 7
	// will lead to testKey4.
	path := sha256.Sum256([]byte("testKey2"))
	flipPathBit(path[:], 3)
	flipPathBit(path[:], 6)
	closestKey, closestValueHash, closestSum, proof, err = smst.ProveClosest(path[:])
	require.NoError(t, err)
	require.NotEqual(t, proof, &SparseMerkleProof{})

	result = VerifySumProof(proof, root, closestKey, closestValueHash, closestSum, NoPrehashSpec(sha256.New(), true))
	require.True(t, result)
	closestPath := sha256.Sum256([]byte("testKey2"))
	require.Equal(t, closestPath[:], closestKey)
	require.Equal(t, []byte("testValue2"), closestValueHash)
	require.Equal(t, closestSum, uint64(24))

	// testKey4 is the neighbour of testKey2, by flipping the final bit of the
	// extension node we change the longest common prefix to that of testKey4
	path2 := sha256.Sum256([]byte("testKey2"))
	flipPathBit(path2[:], 3)
	flipPathBit(path2[:], 7)
	closestKey, closestValueHash, closestSum, proof, err = smst.ProveClosest(path2[:])
	require.NoError(t, err)
	require.NotEqual(t, proof, &SparseMerkleProof{})

	result = VerifySumProof(proof, root, closestKey, closestValueHash, closestSum, NoPrehashSpec(sha256.New(), true))
	require.True(t, result)
	closestPath = sha256.Sum256([]byte("testKey4"))
	require.Equal(t, closestPath[:], closestKey)
	require.Equal(t, []byte("testValue4"), closestValueHash)
	require.Equal(t, closestSum, uint64(30))

	require.NoError(t, smn.Stop())
}

func TestSMST_ProveClosestEmptyAndOneNode(t *testing.T) {
	var smn KVStore
	var smst *SMST
	var proof *SparseMerkleProof
	var err error
	var closestPath, closestValueHash []byte
	var closestSum uint64

	smn, err = NewKVStore("")
	require.NoError(t, err)
	smst = NewSparseMerkleSumTree(smn, sha256.New(), WithValueHasher(nil))

	path := sha256.Sum256([]byte("testKey2"))
	flipPathBit(path[:], 3)
	flipPathBit(path[:], 6)
	closestPath, closestValueHash, closestSum, proof, err = smst.ProveClosest(path[:])
	require.NoError(t, err)
	require.Equal(t, proof, &SparseMerkleProof{})

	result := VerifySumProof(proof, smst.Root(), closestPath, closestValueHash, closestSum, NoPrehashSpec(sha256.New(), true))
	require.True(t, result)

	require.NoError(t, smst.Update([]byte("foo"), []byte("bar"), 5))
	closestPath, closestValueHash, closestSum, proof, err = smst.ProveClosest(path[:])
	require.NoError(t, err)
	require.Equal(t, proof, &SparseMerkleProof{})
	require.Equal(t, closestValueHash, []byte("bar"))
	require.Equal(t, closestSum, uint64(5))

	result = VerifySumProof(proof, smst.Root(), closestPath, closestValueHash, closestSum, NoPrehashSpec(sha256.New(), true))
	require.True(t, result)

	require.NoError(t, smn.Stop())
}
