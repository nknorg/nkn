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

func dumpState(from *crypto.PubKey, description string, s *State) {
	node := publickKeyToNodeID(from)
	if s == nil {
		log.Infof("From: %d | %s", node, description)
		return
	}
	str := ""
	if s.HasBit(FloodingFinished) {
		str += "FloodingFinished"
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
	log.Infof("From: %d | Current State: %s | %s", node, str, description)
	return
}
