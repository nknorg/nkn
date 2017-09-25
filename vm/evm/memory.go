package evm

import "fmt"

type Memory struct {
	store       []byte
	lastReturn  []byte
}

func NewMemory() *Memory{
	return &Memory{}
}

func (m *Memory) Set(offset, size uint64, value []byte) error {
	m.Resize(offset+size)
	if size > 0 {
		copy(m.store[offset:offset+size], value)
	}
	return nil
}

func (m *Memory) Get(offset, size int64) (cpy []byte) {
	if size == 0 {
		return nil
	}
	if len(m.store) > int(offset) {
		cpy = make([]byte, size)
		copy(cpy, m.store[offset:offset+size])
		return
	}
	return
}

func (m *Memory) GetPtr(offset, size int64) []byte {
	if size == 0 {
		return nil
	}
	if len(m.store) > int(offset) {
		return m.store[offset:offset+size]
	}
	return nil
}

func (m *Memory) Len() int {
	return len(m.store)
}

func (m *Memory) Data() []byte {
	return m.store
}

func (m *Memory) Resize(size uint64) {
	if uint64(m.Len()) < size {
		m.store = append(m.store, make([]byte, size-uint64(m.Len()))...)
	}
}

func (m *Memory) Print() {
	fmt.Printf("### mem %d bytes ###\n", len(m.store))
	if len(m.store) > 0 {
		addr := 0
		for i := 0; i+32 <= len(m.store); i += 32 {
			fmt.Printf("%03d: % x\n", addr, m.store[i:i+32])
			addr++
		}
	} else {
		fmt.Println("-- empty --")
	}
	fmt.Println("####################")
}