package trie

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/nknorg/nkn/common"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/util/log"
)

const (
	TagHashNode        = 0
	TagValueNode       = 1
	TagShortNode       = 2
	TagFullNode        = 17
	LenOfChildrenNodes = 17
)

var indices = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f", "[17]"}

type node interface {
	fString(string) string
	cache() (hashNode, bool)
	Serialize(w io.Writer) error
}

type (
	fullNode struct {
		Children [LenOfChildrenNodes]node // Actual trie node data to encode/decode (needs custom encoder)
		flags    nodeFlag
	}
	shortNode struct {
		Key   []byte
		Val   node
		flags nodeFlag
	}
	hashNode  []byte
	valueNode []byte
)

func (n *fullNode) copy() *fullNode {
	c := *n
	return &c
}
func (n *shortNode) copy() *shortNode {
	c := *n
	return &c
}

func (n *fullNode) cache() (hashNode, bool) {
	return n.flags.hash, n.flags.dirty
}
func (n *shortNode) cache() (hashNode, bool) {
	return n.flags.hash, n.flags.dirty
}
func (n hashNode) cache() (hashNode, bool) {
	return nil, true
}
func (n valueNode) cache() (hashNode, bool) {
	return nil, true
}

func (n *fullNode) fString(ind string) string {
	resp := fmt.Sprintf("[\n%s  ", ind)
	for i, node := range n.Children {
		if node == nil {
			resp += fmt.Sprintf("%s: <nil> ", indices[i])
		} else {
			resp += fmt.Sprintf("%s: %v", indices[i], node.fString(ind+"  "))
		}
	}
	return resp + fmt.Sprintf("\n%s] ", ind)
}

func (n *shortNode) fString(ind string) string {
	return fmt.Sprintf("{%v: %v} ", common.BytesToHexString(n.Key), n.Val.fString(ind+"  "))

}
func (n hashNode) fString(ind string) string {
	return fmt.Sprintf("<%s>", common.BytesToHexString(n))

}
func (n valueNode) fString(ind string) string {
	return fmt.Sprintf("%s", common.BytesToHexString(n))

}

func (n *fullNode) String() string {
	return n.fString("")
}
func (n *shortNode) String() string {
	return n.fString("")
}
func (n hashNode) String() string {
	return n.fString("")
}
func (n valueNode) String() string {
	return n.fString("")
}

type nodeFlag struct {
	hash  hashNode
	dirty bool
}

func mustDecodeNode(hash, buf []byte, needFlags bool) node {
	n, err := decodeNode(hash, buf, needFlags)
	if err != nil {
		log.Fatalf("Trie node %x decode error: %v", hash, err)
	}
	return n
}

func decodeNode(hash, buf []byte, needFlags bool) (node, error) {
	if len(buf) == 0 {
		return nil, io.ErrUnexpectedEOF
	}

	buff := bytes.NewBuffer(buf)
	return Deserialize(hash, buff, needFlags)
}

func (n *fullNode) Serialize(w io.Writer) error {
	if err := serialization.WriteVarUint(w, uint64(TagFullNode)); err != nil {
		return err
	}

	for idx, ns := range n.Children {
		if err := serialization.WriteVarUint(w, uint64(idx)); err != nil {
			return err
		}
		if err := ns.Serialize(w); err != nil {
			return err
		}
	}

	return nil
}

func (n *shortNode) Serialize(w io.Writer) error {
	if err := serialization.WriteVarUint(w, uint64(TagShortNode)); err != nil {
		return err
	}
	if err := serialization.WriteVarBytes(w, n.Key); err != nil {
		return err
	}
	if err := n.Val.Serialize(w); err != nil {
		return err
	}
	return nil
}

func (n hashNode) Serialize(w io.Writer) error {
	if err := serialization.WriteVarUint(w, uint64(TagHashNode)); err != nil {
		return err
	}
	if err := serialization.WriteVarBytes(w, []byte(n)); err != nil {
		return err
	}

	return nil
}

func (n valueNode) Serialize(w io.Writer) error {
	if err := serialization.WriteVarUint(w, uint64(TagValueNode)); err != nil {
		return err
	}

	if err := serialization.WriteVarBytes(w, []byte(n)); err != nil {
		return err
	}

	return nil
}

func Deserialize(hash []byte, r io.Reader, needFlags bool) (node, error) {
	count, err := serialization.ReadVarUint(r, 20)
	if err != nil {
		return nil, err
	}

	switch count {
	case TagHashNode:
		buff, err := serialization.ReadVarBytes(r)
		if err != nil {
			return nil, err
		}
		return hashNode(buff), nil
	case TagValueNode:
		buff, err := serialization.ReadVarBytes(r)
		if err != nil {
			return nil, err
		}
		//fmt.Println("+++++++++++++++++", len(buff), buff, valueNode(buff), valueNode(nil))
		if len(buff) == 0 || buff == nil {
			return nil, nil
		}

		return valueNode(buff), nil
	case TagShortNode:
		var s shortNode
		key, err := serialization.ReadVarBytes(r)
		if err != nil {
			return nil, err
		}
		s.Key = compactToHex(key)

		s.Val, err = Deserialize(hash, r, needFlags)
		if err != nil {
			return nil, err
		}
		if needFlags {
			s.flags.hash = hashNode(hash)
			s.flags.dirty = false
		}

		return &s, nil
	case TagFullNode:
		var f fullNode
		for i := 0; i < LenOfChildrenNodes; i++ {
			idx, err := serialization.ReadVarUint(r, 20)
			if err != nil {
				return nil, err
			}
			if int(idx) != i {
				return nil, errors.New("idex error")
			}
			n, err := Deserialize(hash, r, needFlags)
			if err != nil {
				return nil, err
			}
			f.Children[i] = n
		}
		if needFlags {
			f.flags.hash = hashNode(hash)
			f.flags.dirty = false
		}

		return &f, nil
	}

	return nil, errors.New("errors")
}
