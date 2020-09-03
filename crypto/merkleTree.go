package crypto

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/nknorg/nkn/v2/common"
)

var (
	DOUBLE_SHA256 = func(s []common.Uint256) common.Uint256 {
		b := new(bytes.Buffer)
		for _, d := range s {
			d.Serialize(b)
		}
		temp := sha256.Sum256(b.Bytes())
		f := sha256.Sum256(temp[:])
		return common.Uint256(f)
	}
)

type MerkleTree struct {
	Depth uint
	Root  *MerkleTreeNode
}

type MerkleTreeNode struct {
	Hash  common.Uint256
	Left  *MerkleTreeNode
	Right *MerkleTreeNode
}

func (t *MerkleTreeNode) IsLeaf() bool {
	return t.Left == nil && t.Right == nil
}

//use []Uint256 to create a new MerkleTree
func NewMerkleTree(hashes []common.Uint256) (*MerkleTree, error) {
	if len(hashes) == 0 {
		return nil, errors.New("NewMerkleTree input no item error.")
	}
	var height uint

	height = 1
	nodes := generateLeaves(hashes)
	for len(nodes) > 1 {
		nodes = levelUp(nodes)
		height += 1
	}
	mt := &MerkleTree{
		Root:  nodes[0],
		Depth: height,
	}
	return mt, nil

}

//Generate the leaves nodes
func generateLeaves(hashes []common.Uint256) []*MerkleTreeNode {
	var leaves []*MerkleTreeNode
	for _, d := range hashes {
		node := &MerkleTreeNode{
			Hash: d,
		}
		leaves = append(leaves, node)
	}
	return leaves
}

//calc the next level's hash use double sha256
func levelUp(nodes []*MerkleTreeNode) []*MerkleTreeNode {
	var nextLevel []*MerkleTreeNode
	for i := 0; i < len(nodes)/2; i++ {
		var data []common.Uint256
		data = append(data, nodes[i*2].Hash)
		data = append(data, nodes[i*2+1].Hash)
		hash := DOUBLE_SHA256(data)
		node := &MerkleTreeNode{
			Hash:  hash,
			Left:  nodes[i*2],
			Right: nodes[i*2+1],
		}
		nextLevel = append(nextLevel, node)
	}
	if len(nodes)%2 == 1 {
		var data []common.Uint256
		data = append(data, nodes[len(nodes)-1].Hash)
		data = append(data, nodes[len(nodes)-1].Hash)
		hash := DOUBLE_SHA256(data)
		node := &MerkleTreeNode{
			Hash:  hash,
			Left:  nodes[len(nodes)-1],
			Right: nodes[len(nodes)-1],
		}
		nextLevel = append(nextLevel, node)
	}
	return nextLevel
}

//input a []uint256, create a MerkleTree & calc the root hash
func ComputeRoot(hashes []common.Uint256) (common.Uint256, error) {
	if len(hashes) == 0 {
		return common.Uint256{}, errors.New("NewMerkleTree input no item error.")
	}
	if len(hashes) == 1 {
		return hashes[0], nil
	}
	tree, _ := NewMerkleTree(hashes)
	return tree.Root.Hash, nil
}

func VerifyRoot(txnsHash []common.Uint256, txnsRoot []byte) error {
	if hasDuplicateHash(txnsHash) {
		return fmt.Errorf("transactions contain duplicate hash")
	}

	computedRoot, err := ComputeRoot(txnsHash)
	if err != nil {
		return err
	}

	if !bytes.Equal(computedRoot.ToArray(), txnsRoot) {
		return fmt.Errorf("computed txn root %x is different from txn root in header %x", computedRoot.ToArray(), txnsRoot)
	}

	return nil
}

func hasDuplicateHash(txnsHash []common.Uint256) bool {
	count := make(map[common.Uint256]int, len(txnsHash))
	for _, txnHash := range txnsHash {
		if count[txnHash] > 0 {
			return true
		}
		count[txnHash]++
	}
	return false
}
