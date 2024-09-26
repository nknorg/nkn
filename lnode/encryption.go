package lnode

import (
	"fmt"

	"github.com/nknorg/nkn/v2/crypto/ed25519"
	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nkn/v2/util/log"
	nnetnode "github.com/nknorg/nnet/node"
	"golang.org/x/crypto/nacl/box"
	"google.golang.org/protobuf/proto"
)

func (localNode *LocalNode) ComputeSharedKey(remotePublicKey []byte) (*[node.SharedKeySize]byte, error) {
	if len(remotePublicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key length is %d, expecting %d", len(remotePublicKey), ed25519.PublicKeySize)
	}

	var pk [ed25519.PublicKeySize]byte
	copy(pk[:], remotePublicKey)
	curve25519PublicKey, ok := ed25519.PublicKeyToCurve25519PublicKey(&pk)
	if !ok {
		return nil, fmt.Errorf("converting public key %x to curve25519 public key failed", remotePublicKey)
	}

	var sk [ed25519.PrivateKeySize]byte
	copy(sk[:], localNode.account.PrivateKey)
	curve25519PrivateKey := ed25519.PrivateKeyToCurve25519PrivateKey(&sk)

	var sharedKey [node.SharedKeySize]byte
	box.Precompute(&sharedKey, curve25519PublicKey, curve25519PrivateKey)
	return &sharedKey, nil
}

func (localNode *LocalNode) encryptMessage(msg []byte, rn *nnetnode.RemoteNode) []byte {
	var sharedKey *[node.SharedKeySize]byte
	if remoteNode := localNode.GetNeighborByNNetNode(rn); remoteNode != nil {
		return remoteNode.EncryptMessage(msg)
	}
	return node.EncryptMessage(msg, sharedKey)
}

func (localNode *LocalNode) decryptMessage(msg []byte, rn *nnetnode.RemoteNode) ([]byte, error) {
	var sharedKey *[node.SharedKeySize]byte
	if remoteNode := localNode.GetNeighborByNNetNode(rn); remoteNode != nil {
		decrypted, _, err := remoteNode.DecryptMessage(msg)
		return decrypted, err
	} else if rn.Data != nil {
		nodeData := &pb.NodeData{}
		if err := proto.Unmarshal(rn.Data, nodeData); err == nil {
			if len(nodeData.PublicKey) > 0 {
				sharedKey, err = localNode.ComputeSharedKey(nodeData.PublicKey)
				if err != nil {
					log.Warningf("Compute shared key for pk %x error: %v", nodeData.PublicKey, err)
				}
			}
		}
	}
	decrypted, _, err := node.DecryptMessage(msg, sharedKey)
	return decrypted, err
}
