package packetbuffer

import (
	"encoding/hex"
	"sync"

	"github.com/nknorg/nkn/net/node"
)

// PacketBuffer is the buffer to hold message for clients not online
type PacketBuffer struct {
	sync.Mutex
	buffer map[string][]*node.RelayPacket
}

// NewPacketBuffer creates a PacketBuffer
func NewPacketBuffer() *PacketBuffer {
	return &PacketBuffer{
		buffer: make(map[string][]*node.RelayPacket),
	}
}

// AddPacket adds a packet to packet buffer
func (packetBuffer *PacketBuffer) AddPacket(clientID []byte, packet *node.RelayPacket) {
	if packet.MaxHoldingSeconds == 0 {
		return
	}
	clientIDStr := hex.EncodeToString(clientID)
	packetBuffer.Lock()
	defer packetBuffer.Unlock()
	packetBuffer.buffer[clientIDStr] = append(packetBuffer.buffer[clientIDStr], packet)
}

// PopPackets reads and clears all packets of a client
func (packetBuffer *PacketBuffer) PopPackets(clientID []byte) []*node.RelayPacket {
	clientIDStr := hex.EncodeToString(clientID)
	packetBuffer.Lock()
	defer packetBuffer.Unlock()
	packets := packetBuffer.buffer[clientIDStr]
	packetBuffer.buffer[clientIDStr] = nil
	return packets
}
