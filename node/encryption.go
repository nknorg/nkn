package node

import (
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/nknorg/nkn/crypto/ed25519"
	"github.com/nknorg/nkn/pb"
	"github.com/nknorg/nkn/util/log"
	nnetnode "github.com/nknorg/nnet/node"
	"golang.org/x/crypto/nacl/box"
)

const nonceSize = 24
const sharedKeySize = 32

var emptyNonce [nonceSize]byte

func (localNode *LocalNode) computeSharedKey(remotePublicKey []byte) (*[sharedKeySize]byte, error) {
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

	var sharedKey [sharedKeySize]byte
	box.Precompute(&sharedKey, curve25519PublicKey, curve25519PrivateKey)
	return &sharedKey, nil
}

func (localNode *LocalNode) encryptMessage(msg []byte, rn *nnetnode.RemoteNode) []byte {
	var sharedKey *[sharedKeySize]byte
	if remoteNode := localNode.getNbrByNNetNode(rn); remoteNode != nil {
		sharedKey = remoteNode.sharedKey
	}
	return encryptMessage(msg, sharedKey)
}

func (localNode *LocalNode) decryptMessage(msg []byte, rn *nnetnode.RemoteNode) ([]byte, error) {
	var sharedKey *[sharedKeySize]byte
	if remoteNode := localNode.getNbrByNNetNode(rn); remoteNode != nil {
		sharedKey = remoteNode.sharedKey
	} else if rn.Data != nil {
		nodeData := &pb.NodeData{}
		if err := proto.Unmarshal(rn.Data, nodeData); err == nil {
			if len(nodeData.PublicKey) > 0 {
				sharedKey, err = localNode.computeSharedKey(nodeData.PublicKey[1:])
				if err != nil {
					log.Warningf("Compute shared key for pk %x error: %v", nodeData.PublicKey, err)
				}
			}
		}
	}
	decrypted, _, err := decryptMessage(msg, sharedKey)
	return decrypted, err
}

func encryptMessage(message []byte, sharedKey *[sharedKeySize]byte) []byte {
	if sharedKey == nil {
		return append(emptyNonce[:], message...)
	}

	var nonce [nonceSize]byte
	_, err := rand.Read(nonce[:])
	if err != nil {
		return nil
	}

	encrypted := make([]byte, len(message)+box.Overhead+nonceSize)
	copy(encrypted[:nonceSize], nonce[:])
	box.SealAfterPrecomputation(encrypted[nonceSize:nonceSize], message, &nonce, sharedKey)

	return encrypted
}

func decryptMessage(message []byte, sharedKey *[sharedKeySize]byte) ([]byte, bool, error) {
	if len(message) < nonceSize {
		return nil, false, fmt.Errorf("encrypted message should have at least %d bytes", nonceSize)
	}

	var nonce [nonceSize]byte
	copy(nonce[:], message[:nonceSize])
	if nonce == emptyNonce {
		return message[nonceSize:], false, nil
	}

	if sharedKey == nil {
		return nil, false, errors.New("cannot decrypt message: no shared key yet")
	}

	if len(message) < nonceSize+box.Overhead {
		return nil, false, fmt.Errorf("encrypted message should have at least %d bytes", nonceSize+box.Overhead)
	}

	decrypted := make([]byte, len(message)-nonceSize-box.Overhead)
	_, ok := box.OpenAfterPrecomputation(decrypted[:0], message[nonceSize:], &nonce, sharedKey)
	if !ok {
		return nil, false, errors.New("decrypt message failed")
	}

	return decrypted, true, nil
}
