package signature

//Signer is the abstract interface of user's information(Keys) for signing data.
type Signer interface {
	PrivKey() []byte
	PubKey() []byte
}
