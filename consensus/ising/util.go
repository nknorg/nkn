package ising

import (
	"bytes"
	"encoding/binary"

	"github.com/nknorg/nkn/crypto"
	"github.com/nknorg/nkn/util/log"
)

func publickKeyToNodeID(pubKey *crypto.PubKey) uint64 {
	var id uint64
	key, err := pubKey.EncodePoint(true)
	if err != nil {
		log.Error(err)
	}
	err = binary.Read(bytes.NewBuffer(key[:8]), binary.LittleEndian, &id)
	if err != nil {
		log.Error(err)
	}

	return id
}

func dumpState(from *crypto.PubKey, description string, s State) {
	str := ""
	if s.HasBit(InitialState) {
		str += "InitialState"
	}
	if s.HasBit(FloodingFinished) {
		str += " -> FloodingFinished"
	}
	if s.HasBit(RequestSent) {
		str += " -> RequestSent"
	}
	if s.HasBit(ProposalSent) {
		str += " -> ProposalSent"
	}
	if s.HasBit(OpinionSent) {
		str += " -> OpinionSent"
	}
	node := publickKeyToNodeID(from)
	log.Infof("From: %d | Current State: %s | %s", node, str, description)
}
