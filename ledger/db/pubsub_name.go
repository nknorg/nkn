package db

import (
	"bytes"

	"github.com/nknorg/nkn/common/serialization"
	"github.com/nknorg/nkn/util/address"
)

func (cs *ChainStore) SaveName(registrant []byte, name string) error {
	// generate registrant key
	registrantKey := bytes.NewBuffer(nil)
	registrantKey.WriteByte(byte(NS_Registrant))
	serialization.WriteVarBytes(registrantKey, registrant)

	// generate name key
	nameKey := bytes.NewBuffer(nil)
	nameKey.WriteByte(byte(NS_Name))
	serialization.WriteVarString(nameKey, name)

	// PUT VALUE
	w := bytes.NewBuffer(nil)
	serialization.WriteVarString(w, name)
	err := cs.st.BatchPut(registrantKey.Bytes(), w.Bytes())
	if err != nil {
		return err
	}

	w = bytes.NewBuffer(nil)
	serialization.WriteVarBytes(w, registrant)
	err = cs.st.BatchPut(nameKey.Bytes(), w.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) DeleteName(registrant []byte) error {
	name, err := cs.GetName(registrant)
	if err != nil {
		return err
	}

	// generate registrant key
	registrantKey := bytes.NewBuffer(nil)
	registrantKey.WriteByte(byte(NS_Registrant))
	serialization.WriteVarBytes(registrantKey, registrant)

	// generate name key
	nameKey := bytes.NewBuffer(nil)
	nameKey.WriteByte(byte(NS_Name))
	serialization.WriteVarString(nameKey, *name)

	// DELETE VALUE
	err = cs.st.BatchDelete(registrantKey.Bytes())
	if err != nil {
		return err
	}
	err = cs.st.BatchDelete(nameKey.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) GetName(registrant []byte) (*string, error) {
	// generate key
	registrantKey := bytes.NewBuffer(nil)
	registrantKey.WriteByte(byte(NS_Registrant))
	serialization.WriteVarBytes(registrantKey, registrant)

	data, err := cs.st.Get(registrantKey.Bytes())
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)

	name, err := serialization.ReadVarString(r)
	if err != nil {
		return nil, err
	}

	return &name, nil
}

func (cs *ChainStore) GetRegistrant(name string) ([]byte, error) {
	// generate key
	nameKey := bytes.NewBuffer(nil)
	nameKey.WriteByte(byte(NS_Name))
	serialization.WriteVarString(nameKey, name)

	data, err := cs.st.Get(nameKey.Bytes())
	if err != nil {
		return nil, err
	}

	r := bytes.NewReader(data)

	registrant, err := serialization.ReadVarBytes(r)
	if err != nil {
		return nil, err
	}

	return registrant, nil
}

func generateSubscriberKey(subscriber []byte, identifier string, topic string) []byte {
	subscriberKey := bytes.NewBuffer(nil)
	subscriberKey.WriteByte(byte(PS_Topic))
	serialization.WriteVarString(subscriberKey, topic)
	serialization.WriteVarBytes(subscriberKey, subscriber)
	serialization.WriteVarString(subscriberKey, identifier)

	return subscriberKey.Bytes()
}

func (cs *ChainStore) Subscribe(subscriber []byte, identifier string, topic string, duration uint32, height uint32) error {
	if duration == 0 {
		return nil
	}

	subscriberKey := generateSubscriberKey(subscriber, identifier, topic)

	// PUT VALUE
	err := cs.st.BatchPut(subscriberKey, []byte{})
	if err != nil {
		return err
	}

	err = cs.ExpireKeyAtBlock(height+duration, subscriberKey)
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) Unsubscribe(subscriber []byte, identifier string, topic string, duration uint32, height uint32) error {
	if duration == 0 {
		return nil
	}

	subscriberKey := generateSubscriberKey(subscriber, identifier, topic)

	// DELETE VALUE
	err := cs.st.BatchDelete(subscriberKey)
	if err != nil {
		return err
	}

	err = cs.CancelKeyExpirationAtBlock(height+duration, subscriberKey)
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) IsSubscribed(subscriber []byte, identifier string, topic string) (bool, error) {
	subscriberKey := generateSubscriberKey(subscriber, identifier, topic)

	return cs.st.Has(subscriberKey)
}

func (cs *ChainStore) GetSubscribers(topic string) []string {
	subscribers := make([]string, 0)

	prefix := append([]byte{byte(PS_Topic), byte(len(topic))}, []byte(topic)...)
	iter := cs.st.NewIterator(prefix)
	for iter.Next() {
		rk := bytes.NewReader(iter.Key())

		// read prefix
		_, _ = serialization.ReadBytes(rk, uint64(len(prefix)))

		subscriber, _ := serialization.ReadVarBytes(rk)
		identifier, _ := serialization.ReadVarString(rk)
		subscriberString := address.MakeAddressString(subscriber, identifier)

		subscribers = append(subscribers, subscriberString)
	}

	return subscribers
}

func (cs *ChainStore) GetSubscribersCount(topic string) int {
	subscribers := 0

	prefix := append([]byte{byte(PS_Topic), byte(len(topic))}, []byte(topic)...)
	iter := cs.st.NewIterator(prefix)
	for iter.Next() {
		subscribers++
	}

	return subscribers
}

func (cs *ChainStore) ExpireKeyAtBlock(height uint32, key []byte) error {
	expireKey := bytes.NewBuffer(nil)
	expireKey.WriteByte(byte(SYS_ExpireKey))
	serialization.WriteUint32(expireKey, height)
	serialization.WriteVarBytes(expireKey, key)

	err := cs.st.BatchPut(expireKey.Bytes(), []byte{})
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) CancelKeyExpirationAtBlock(height uint32, key []byte) error {
	expireKey := bytes.NewBuffer(nil)
	expireKey.WriteByte(byte(SYS_ExpireKey))
	serialization.WriteUint32(expireKey, height)
	serialization.WriteVarBytes(expireKey, key)

	err := cs.st.BatchDelete(expireKey.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (cs *ChainStore) GetExpiredKeys(height uint32) [][]byte {
	keys := make([][]byte, 0)

	prefix := bytes.NewBuffer(nil)
	prefix.WriteByte(byte(SYS_ExpireKey))
	serialization.WriteUint32(prefix, height)

	iter := cs.st.NewIterator(prefix.Bytes())
	for iter.Next() {
		key := make([]byte, len(iter.Key()))
		copy(key, iter.Key())
		keys = append(keys, key)
	}

	return keys
}

func (cs *ChainStore) RemoveExpiredKey(key []byte) error {
	rk := bytes.NewReader(key)

	// read prefix
	_, err := serialization.ReadBytes(rk, 5)
	if err != nil {
		return err
	}

	expiredKey, err := serialization.ReadVarBytes(rk)
	if err != nil {
		return err
	}

	err = cs.st.BatchDelete(key)
	if err != nil {
		return err
	}

	err = cs.st.BatchDelete(expiredKey)
	if err != nil {
		return err
	}

	return nil
}
