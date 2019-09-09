package store

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/nknorg/nkn/chain/trie"
	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/util/address"
)

const hashPrefixLength = 20

type pubSub struct {
	subscriber []byte
	identifier string
	meta       string
	expiresAt  uint32
}

type pubSubCleanup map[string]struct{}

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

func getPubSubId(topic string, bucket uint32, subscriber []byte, identifier string) []byte {
	pubSubId := bytes.NewBuffer(nil)

	pubSubId.Write(getTopicBucketId(topic, bucket))

	subscriberHash := sha256.Sum256(append(subscriber, identifier...))
	pubSubId.Write(subscriberHash[:hashPrefixLength])

	return pubSubId.Bytes()
}

func getPubSubCleanupId(height uint32) []byte {
	buf := new(bytes.Buffer)
	_ = serialization.WriteUint32(buf, height)
	return buf.Bytes()
}

func (sdb *StateDB) getPubSub(id []byte) (*pubSub, error) {
	if v, ok := sdb.pubSub.Load(string(id)); ok {
		if ps, ok := v.(*pubSub); ok {
			return ps, nil
		}
	}

	enc, err := sdb.trie.TryGet(append(PubSubPrefix, id...))
	if err != nil {
		return nil, err
	}

	ps := &pubSub{}

	if len(enc) > 0 {
		buff := bytes.NewBuffer(enc)
		if err := ps.Deserialize(buff); err != nil {
			return nil, fmt.Errorf("[getPubSub]Failed to decode state object for pub sub: %v", err)
		}
	}

	sdb.pubSub.Store(string(id), ps)

	return ps, nil
}

func (sdb *StateDB) getPubSubCleanup(height uint32) (pubSubCleanup, error) {
	if v, ok := sdb.pubSubCleanup.Load(height); ok {
		if psc, ok := v.(pubSubCleanup); ok {
			return psc, nil
		}
	}

	enc, err := sdb.trie.TryGet(append(PubSubCleanupPrefix, getPubSubCleanupId(height)...))
	if err != nil {
		return nil, fmt.Errorf("[getPubSubCleanup]can not get pub sub cleanup from trie: %v", err)
	}

	psc := make(pubSubCleanup, 0)

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

	sdb.pubSubCleanup.Store(height, psc)

	return psc, nil
}

func (sdb *StateDB) cleanupPubSubAtHeight(height uint32, id []byte) error {
	ids, err := sdb.getPubSubCleanup(height)
	if err != nil {
		return err
	}
	ids[string(id)] = struct{}{}
	return nil
}

func (sdb *StateDB) cancelPubSubCleanupAtHeight(height uint32, id []byte) error {
	ids, err := sdb.getPubSubCleanup(height)
	if err != nil {
		return err
	}
	if _, ok := ids[string(id)]; ok {
		delete(ids, string(id))
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

func (sdb *StateDB) unsubscribe(topic string, subscriber []byte, identifier string) error {
	id := getPubSubId(topic, 0, subscriber, identifier)

	ps, err := sdb.getPubSub(id)
	if err != nil {
		return err
	}

	if !ps.Empty() {
		if err := sdb.cancelPubSubCleanupAtHeight(ps.expiresAt, id); err != nil {
			return err
		}
		sdb.pubSub.Store(string(id), nil)
	}

	return nil
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

func (sdb *StateDB) getSubscription(topic string, bucket uint32, subscriber []byte, identifier string) (string, uint32, error) {
	id := getPubSubId(topic, bucket, subscriber, identifier)

	ps, err := sdb.getPubSub(id)
	if err != nil {
		return "", 0, err
	}

	return ps.meta, ps.expiresAt, nil
}

func (cs *ChainStore) GetSubscription(topic string, bucket uint32, subscriber []byte, identifier string) (string, uint32, error) {
	return cs.States.getSubscription(topic, bucket, subscriber, identifier)
}

func (sdb *StateDB) getSubscribers(topic string, bucket, offset, limit uint32) (map[string]string, error) {
	subscribers := make(map[string]string, 0)

	prefix := string(append(PubSubPrefix, getTopicBucketId(topic, bucket)...))
	iter := trie.NewIterator(sdb.trie.NodeIterator([]byte(prefix)))
	i := uint32(0)
	for ; iter.Next(); i++ {
		if !strings.HasPrefix(string(iter.Key), prefix) {
			break
		}
		if i < offset {
			continue
		}
		if limit > 0 && i >= offset + limit {
			break
		}

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

func (cs *ChainStore) GetSubscribers(topic string, bucket, offset, limit uint32) ([]string, error) {
	subscribersWithMeta, err := cs.States.getSubscribers(topic, bucket, offset, limit)
	if err != nil {
		return nil, err
	}
	subscribers := make([]string, 0)
	for subscriber, _ := range subscribersWithMeta {
		subscribers = append(subscribers, subscriber)
	}
	return subscribers, nil
}

func (cs *ChainStore) GetSubscribersWithMeta(topic string, bucket, offset, limit uint32) (map[string]string, error) {
	return cs.States.getSubscribers(topic, bucket, offset, limit)
}

func (sdb *StateDB) getSubscribersCount(topic string, bucket uint32) int {
	subscribers := 0

	prefix := string(append(PubSubPrefix, getTopicBucketId(topic, bucket)...))
	iter := trie.NewIterator(sdb.trie.NodeIterator([]byte(prefix)))
	for iter.Next() {
		if !strings.HasPrefix(string(iter.Key), prefix) {
			break
		}

		subscribers++
	}

	return subscribers
}

func (cs *ChainStore) GetSubscribersCount(topic string, bucket uint32) int {
	return cs.States.getSubscribersCount(topic, bucket)
}

func (sdb *StateDB) deletePubSub(id string) error {
	err := sdb.trie.TryDelete(append(PubSubPrefix, id...))
	if err != nil {
		return err
	}

	sdb.pubSub.Delete(id)
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

	sdb.pubSubCleanup.Delete(height)
	return nil
}

func (sdb *StateDB) updatePubSubCleanup(height uint32, psc pubSubCleanup) error {
	buff := bytes.NewBuffer(nil)

	if err := serialization.WriteVarUint(buff, uint64(len(psc))); err != nil {
		panic(fmt.Errorf("can't encode pub sub cleanup %v: %v", psc, err))
	}
	pscs := make([]string, 0)
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
		sdb.pubSub.Store(id, nil)
	}
	sdb.pubSubCleanup.Store(height, nil)

	return nil
}

func (sdb *StateDB) FinalizePubSub(commit bool) {
	sdb.pubSub.Range(func(key, value interface{}) bool {
		if id, ok := key.(string); ok {
			if ps, ok := value.(*pubSub); ok && !ps.Empty() {
				sdb.updatePubSub(id, ps)
			} else {
				sdb.deletePubSub(id)
			}
			if commit {
				sdb.pubSub.Delete(id)
			}
		}
		return true
	})

	sdb.pubSubCleanup.Range(func(key, value interface{}) bool {
		if height, ok := key.(uint32); ok {
			if psc, ok := value.(pubSubCleanup); ok && len(psc) > 0 {
				sdb.updatePubSubCleanup(height, psc)
			} else {
				sdb.deletePubSubCleanup(height)
			}
			if commit {
				sdb.pubSubCleanup.Delete(height)
			}
		}
		return true
	})
}
