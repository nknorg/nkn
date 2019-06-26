package db

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/nknorg/nkn/chain/trie"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/transaction"
	"github.com/nknorg/nkn/util/address"
)

const hashPrefixLength = 20

type pubSub struct {
	subscriber []byte
	identifier string
	meta       string
	expiresAt  uint32
}

func (ps *pubSub) Serialize(w io.Writer) error {
	if err := serialization.WriteVarBytes(w, ps.subscriber); err != nil {
		return err
	}
	if err := serialization.WriteVarString(w, ps.identifier); err != nil {
		return err
	}
	if err := serialization.WriteVarString(w, ps.meta); err != nil {
		return err
	}

	if err := serialization.WriteUint32(w, ps.expiresAt); err != nil {
		return err
	}

	return nil
}

func (ps *pubSub) Deserialize(r io.Reader) error {
	var err error

	ps.subscriber, err = serialization.ReadVarBytes(r)
	if err != nil {
		return err
	}

	ps.identifier, err = serialization.ReadVarString(r)
	if err != nil {
		return err
	}

	ps.meta, err = serialization.ReadVarString(r)
	if err != nil {
		return err
	}

	ps.expiresAt, err = serialization.ReadUint32(r)
	if err != nil {
		return err
	}

	return nil
}

func (ps *pubSub) Empty() bool {
	return len(ps.subscriber) == 0 && len(ps.identifier) == 0 && len(ps.meta) == 0 && ps.expiresAt == 0
}

func getTopicId(topic string) []byte {
	topicHash := sha256.Sum256([]byte(strings.ToLower(topic)))
	return topicHash[:hashPrefixLength]
}

func getTopicBucketId(topic string, bucket uint32) []byte {
	topicBucketId := bytes.NewBuffer(nil)

	topicId := getTopicId(topic)
	topicBucketId.Write(topicId)

	serialization.WriteUint32(topicBucketId, bucket)

	return topicBucketId.Bytes()
}

func getPubSubId(topic string, bucket uint32, subscriber []byte, identifier string) string {
	pubSubId := bytes.NewBuffer(nil)

	pubSubId.Write(getTopicBucketId(topic, bucket))

	subscriberHash := sha256.Sum256(append(subscriber, identifier...))
	pubSubId.Write(subscriberHash[:hashPrefixLength])

	return string(pubSubId.Bytes())
}

func getPubSubCleanupId(height uint32) string {
	buf := new(bytes.Buffer)
	_ = serialization.WriteUint32(buf, height)
	return string(buf.Bytes())
}

func (sdb *StateDB) getPubSub(id string) (*pubSub, error) {
	var ps *pubSub
	var ok bool
	if ps, ok = sdb.pubSub[id]; !ok {
		enc, err := sdb.trie.TryGet(append(PubSubPrefix, id...))
		if err != nil {
			return nil, err
		}

		ps = &pubSub{}

		if len(enc) > 0 {
			buff := bytes.NewBuffer(enc)
			if err := ps.Deserialize(buff); err != nil {
				return nil, fmt.Errorf("[getPubSub]Failed to decode state object for pub sub: %v", err)
			}
		}

		sdb.pubSub[id] = ps
	}

	return ps, nil
}

func (sdb *StateDB) getPubSubCleanup(height uint32) (map[string]struct{}, error) {
	var psc map[string]struct{}
	var ok bool
	if psc, ok = sdb.pubSubCleanup[height]; !ok {
		enc, err := sdb.trie.TryGet(append(PubSubCleanupPrefix, getPubSubCleanupId(height)...))
		if err != nil {
			return nil, fmt.Errorf("[getPubSubCleanup]can not get pub sub cleanup from trie: %v", err)
		}

		psc = make(map[string]struct{}, 0)

		if len(enc) > 0 {
			buff := bytes.NewBuffer(enc)
			pscLength, err := serialization.ReadVarUint(buff, 0)
			if err != nil {
				return nil, fmt.Errorf("[getPubSubCleanup]Failed to decode state object for pub sub cleanup: %v", err)
			}
			for i := uint64(0); i < pscLength; i++ {
				id, err := serialization.ReadVarString(buff)
				if err != nil {
					return nil, fmt.Errorf("[getPubSubCleanup]Failed to decode state object for pub sub cleanup: %v", err)
				}
				psc[id] = struct{}{}
			}
		}

		sdb.pubSubCleanup[height] = psc
	}

	return psc, nil
}

func (sdb *StateDB) cleanupPubSubAtHeight(height uint32, id string) error {
	ids, err := sdb.getPubSubCleanup(height)
	if err != nil {
		return err
	}
	ids[id] = struct{}{}
	return nil
}

func (sdb *StateDB) cancelPubSubCleanupAtHeight(height uint32, id string) error {
	ids, err := sdb.getPubSubCleanup(height)
	if err != nil {
		return err
	}
	if _, ok := ids[id]; ok {
		delete(ids, id)
	}
	return nil
}

func (sdb *StateDB) subscribe(topic string, bucket uint32, subscriber []byte, identifier string, meta string, expiresAt uint32) error {
	id := getPubSubId(topic, bucket, subscriber, identifier)

	ps, err := sdb.getPubSub(id)
	if err != nil {
		return err
	}

	if ps.Empty() {
		ps.subscriber = subscriber
		ps.identifier = identifier
	} else {
		if err := sdb.cancelPubSubCleanupAtHeight(ps.expiresAt, id); err != nil {
			return err
		}
	}
	if err := sdb.cleanupPubSubAtHeight(expiresAt, id); err != nil {
		return err
	}
	ps.meta = meta
	ps.expiresAt = expiresAt

	return nil
}

func (cs *ChainStore) Subscribe(topic string, bucket uint32, subscriber []byte, identifier string, meta string, expiresAt uint32) error {
	return cs.States.subscribe(topic, bucket, subscriber, identifier, meta, expiresAt)
}

func (sdb *StateDB) isSubscribed(topic string, bucket uint32, subscriber []byte, identifier string) (bool, error) {
	id := getPubSubId(topic, bucket, subscriber, identifier)

	ps, err := sdb.getPubSub(id)
	if err != nil {
		return false, err
	}

	return !ps.Empty(), nil
}

func (cs *ChainStore) IsSubscribed(topic string, bucket uint32, subscriber []byte, identifier string) (bool, error) {
	return cs.States.isSubscribed(topic, bucket, subscriber, identifier)
}

func (sdb *StateDB) getSubscribers(topic string, bucket uint32) (map[string]string, error) {
	subscribers := make(map[string]string, 0)

	prefix := getTopicBucketId(topic, bucket)
	iter := trie.NewIterator(sdb.trie.NodeIterator(prefix))
	for iter.Next() {
		buff := bytes.NewBuffer(iter.Value)
		ps := &pubSub{}
		if err := ps.Deserialize(buff); err != nil {
			return nil, err
		}

		subscriberString := address.MakeAddressString(ps.subscriber, ps.identifier)

		subscribers[subscriberString] = ps.meta
	}

	return subscribers, nil
}

func (cs *ChainStore) GetSubscribers(topic string, bucket uint32) (map[string]string, error) {
	return cs.States.getSubscribers(topic, bucket)
}

func (sdb *StateDB) getSubscribersCount(topic string, bucket uint32) int {
	subscribers := 0

	prefix := append(PubSubPrefix, getTopicBucketId(topic, bucket)...)
	iter := trie.NewIterator(sdb.trie.NodeIterator(prefix))
	for iter.Next() {
		subscribers++
	}

	return subscribers
}

func (cs *ChainStore) GetSubscribersCount(topic string, bucket uint32) int {
	return cs.States.getSubscribersCount(topic, bucket)
}

func (sdb *StateDB) getFirstAvailableTopicBucket(topic string) int {
	for i := uint32(0); i < transaction.BucketsLimit; i++ {
		count := sdb.getSubscribersCount(topic, i)
		if count < transaction.SubscriptionsLimit {
			return int(i)
		}
	}

	return -1
}

func (cs *ChainStore) GetFirstAvailableTopicBucket(topic string) int {
	return cs.States.getFirstAvailableTopicBucket(topic)
}

func (sdb *StateDB) getTopicBucketsCount(topic string) (uint32, error) {
	prefix := append(PubSubPrefix, getTopicId(topic)...)

	iter := trie.NewIterator(sdb.trie.NodeIterator(prefix))
	var lastKey []byte
	for iter.Next() {
		lastKey = iter.Key
	}

	rk := bytes.NewReader(lastKey)

	// read prefix
	_, err := serialization.ReadBytes(rk, uint64(len(prefix)))
	if err != nil {
		return 0, err
	}

	return serialization.ReadUint32(rk)
}

func (cs *ChainStore) GetTopicBucketsCount(topic string) (uint32, error) {
	return cs.States.getTopicBucketsCount(topic)
}

func (sdb *StateDB) deletePubSub(id string) error {
	err := sdb.trie.TryDelete(append(PubSubPrefix, id...))
	if err != nil {
		return err
	}

	delete(sdb.pubSub, id)
	return nil
}

func (sdb *StateDB) updatePubSub(id string, pubSub *pubSub) error {
	buff := bytes.NewBuffer(nil)
	err := pubSub.Serialize(buff)
	if err != nil {
		panic(fmt.Errorf("can't encode pub sub %v: %v", pubSub, err))
	}

	return sdb.trie.TryUpdate(append(PubSubPrefix, id...), buff.Bytes())
}

func (sdb *StateDB) deletePubSubCleanup(height uint32) error {
	err := sdb.trie.TryDelete(append(PubSubCleanupPrefix, getPubSubCleanupId(height)...))
	if err != nil {
		return err
	}

	delete(sdb.pubSubCleanup, height)
	return nil
}

func (sdb *StateDB) updatePubSubCleanup(height uint32, psc map[string]struct{}) error {
	buff := bytes.NewBuffer(nil)

	if err := serialization.WriteVarUint(buff, uint64(len(psc))); err != nil {
		panic(fmt.Errorf("can't encode pub sub cleanup %v: %v", psc, err))
	}
	pscs := make([]string, len(psc))
	for id := range psc {
		pscs = append(pscs, id)
	}
	sort.Strings(pscs)
	for _, id := range pscs {
		if err := serialization.WriteVarString(buff, id); err != nil {
			panic(fmt.Errorf("can't encode pub sub cleanup %v: %v", psc, err))
		}
	}

	return sdb.trie.TryUpdate(append(PubSubCleanupPrefix, getPubSubCleanupId(height)...), buff.Bytes())
}

func (sdb *StateDB) CleanupPubSub(height uint32) error {
	ids, err := sdb.getPubSubCleanup(height)
	if err != nil {
		return err
	}
	for id := range ids {
		sdb.pubSub[id] = nil
	}
	sdb.pubSubCleanup[height] = nil

	return nil
}

func (sdb *StateDB) FinalizePubSub(commit bool) {
	for id, pubSub := range sdb.pubSub {
		if pubSub == nil || pubSub.Empty() {
			sdb.deletePubSub(id)
		} else {
			sdb.updatePubSub(id, pubSub)
		}
		if commit {
			delete(sdb.pubSub, id)
		}
	}

	for height, psc := range sdb.pubSubCleanup {
		if psc == nil || len(psc) == 0 {
			sdb.deletePubSubCleanup(height)
		} else {
			sdb.updatePubSubCleanup(height, psc)
		}
		if commit {
			delete(sdb.pubSubCleanup, height)
		}
	}
}
