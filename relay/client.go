package relay

type Client interface {
	GetID() []byte
	GetPubKey() []byte
	Send(data []byte) error
}
