package por

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/nknorg/nkn/v2/config"
	"github.com/nknorg/nkn/v2/crypto"
	"github.com/nknorg/nkn/v2/pb"
	"github.com/nknorg/nnet/overlay/chord"
)

func VerifySigChain(sc *pb.SigChain, height uint32) error {
	if err := VerifySigChainMeta(sc, height); err != nil {
		return err
	}

	if err := VerifySigChainPath(sc, height); err != nil {
		return err
	}

	if err := VerifySigChainSignatures(sc); err != nil {
		return err
	}

	return nil
}

func VerifySigChainMeta(sc *pb.SigChain, height uint32) error {
	if sc.Length() < 3 {
		return fmt.Errorf("sigchain should have at least 3 elements, but only has %d", sc.Length())
	}

	if len(sc.Elems[0].Id) > 0 && !bytes.Equal(sc.SrcId, sc.Elems[0].Id) {
		return fmt.Errorf("sigchain has wrong src id")
	}

	if !sc.IsComplete() {
		return fmt.Errorf("sigchain is not complete")
	}

	if bytes.Equal(sc.Elems[0].Id, sc.Elems[1].Id) {
		return fmt.Errorf("src and first relayer has the same ID")
	}

	if bytes.Equal(sc.Elems[sc.Length()-1].Id, sc.Elems[sc.Length()-2].Id) {
		return fmt.Errorf("dest and last relayer has the same ID")
	}

	if len(sc.Elems[sc.Length()-1].NextPubkey) > 0 {
		return fmt.Errorf("next pubkey in last sigchain elem should be empty")
	}

	if sc.Elems[0].Mining {
		return fmt.Errorf("first sigchain element should have mining set to false")
	}

	if sc.Elems[sc.Length()-1].Mining {
		return fmt.Errorf("last sigchain element should have mining set to false")
	}

	for i, e := range sc.Elems {
		if i == 0 || i == sc.Length()-1 {
			if len(e.Vrf) > 0 {
				return fmt.Errorf("sigchain elem %d vrf should be empty", i)
			}
			if len(e.Proof) > 0 {
				return fmt.Errorf("sigchain elem %d proof should be empty", i)
			}
		} else {
			if len(e.Vrf) == 0 {
				return fmt.Errorf("sigchain elem %d vrf should not be empty", i)
			}
			if len(e.Proof) == 0 {
				return fmt.Errorf("sigchain elem %d proof should not be empty", i)
			}
		}
		if !config.AllowSigChainHashSignature.GetValueAtHeight(height) {
			if e.SigAlgo != pb.SigAlgo_SIGNATURE {
				return fmt.Errorf("sigchain elem %d sig algo should be %v", i, pb.SigAlgo_SIGNATURE)
			}
		}
	}

	return nil
}

// VerifySigChainSignatures returns whether all signatures in sigchain are valid
func VerifySigChainSignatures(sc *pb.SigChain) error {
	prevNextPubkey := sc.SrcPubkey
	buff := bytes.NewBuffer(nil)
	err := sc.SerializationMetadata(buff)
	if err != nil {
		return err
	}
	metaHash := sha256.Sum256(buff.Bytes())
	prevHash := metaHash[:]
	for i, e := range sc.Elems {
		err := crypto.CheckPublicKey(prevNextPubkey)
		if err != nil {
			return fmt.Errorf("invalid pubkey %x: %v", prevNextPubkey, err)
		}

		if sc.IsComplete() && i == sc.Length()-1 {
			h := sha256.Sum256(prevHash)
			prevHash = h[:]
		}

		hash, err := e.Hash(prevHash)
		if err != nil {
			return err
		}

		switch e.SigAlgo {
		case pb.SigAlgo_SIGNATURE:
			err = crypto.Verify(prevNextPubkey, hash, e.Signature)
			if err != nil {
				return fmt.Errorf("signature %x is invalid: %v", e.Signature, err)
			}
		case pb.SigAlgo_HASH:
			if !bytes.Equal(e.Signature, hash) {
				return fmt.Errorf("signature %x is different from expected value %x", e.Signature, hash[:])
			}
		default:
			return fmt.Errorf("unknown SigAlgo %v", e.SigAlgo)
		}

		if len(e.Vrf) > 0 {
			ok := crypto.VerifyVrf(prevNextPubkey, sc.BlockHash, e.Vrf, e.Proof)
			if !ok {
				return fmt.Errorf("invalid vrf or proof")
			}
			prevHash = hash
		} else {
			prevHash = e.Signature
		}

		prevNextPubkey = e.NextPubkey
		if sc.IsComplete() && i == sc.Length()-2 && len(e.NextPubkey) == 0 {
			prevNextPubkey = sc.DestPubkey
		}
	}

	return nil
}

func VerifySigChainPath(sc *pb.SigChain, height uint32) error {
	var t big.Int
	lastNodeID := sc.Elems[sc.Length()-2].Id
	prevDistance := chord.Distance(sc.Elems[1].Id, lastNodeID, config.NodeIDBytes*8)
	for i := 2; i < sc.Length()-1; i++ {
		dist := chord.Distance(sc.Elems[i].Id, lastNodeID, config.NodeIDBytes*8)
		if dist.Cmp(prevDistance) == 0 {
			return fmt.Errorf("relayer %d and %d has the same ID", i-1, i)
		}
		(&t).Mul(dist, big.NewInt(2))
		if t.Cmp(prevDistance) > 0 {
			return fmt.Errorf("signature chain path is invalid")
		}
		prevDistance = dist
	}

	if config.SigChainVerifyFingerTableRange.GetValueAtHeight(height) {
		// only needs to verify node to node hop, and no need to check last node to
		// node hop because it could be successor
		for i := 1; i < sc.Length()-3; i++ {
			dist := chord.Distance(sc.Elems[i].Id, sc.DestId, config.NodeIDBytes*8)
			fingerIdx := dist.BitLen() - 1
			if fingerIdx < 0 {
				return fmt.Errorf("invalid finger table index")
			}
			fingerStartID := chord.PowerOffset(sc.Elems[i].Id, uint32(fingerIdx), config.NodeIDBytes*8)
			fingerEndID := chord.PowerOffset(sc.Elems[i].Id, uint32(fingerIdx+1), config.NodeIDBytes*8)
			if !chord.BetweenLeftIncl(fingerStartID, fingerEndID, sc.Elems[sc.Length()-2].Id) && fingerIdx > 1 {
				fingerStartID = chord.PowerOffset(sc.Elems[i].Id, uint32(fingerIdx-1), config.NodeIDBytes*8)
				fingerEndID = chord.PowerOffset(sc.Elems[i].Id, uint32(fingerIdx), config.NodeIDBytes*8)
			}
			if !chord.BetweenLeftIncl(fingerStartID, fingerEndID, sc.Elems[i+1].Id) {
				return fmt.Errorf("next hop is not in finger table range")
			}
		}
	}

	return nil
}
