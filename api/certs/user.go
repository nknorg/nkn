package certs

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/nknorg/nkn/v2/node"
	"github.com/nknorg/nkn/v2/config"

	"github.com/go-acme/lego/v3/registration"
)

// You'll need a User or account type that implements acme.User
type User struct {
	Domain       string
	Email        string
	Registration *registration.Resource
	Key          *rsa.PrivateKey
}

func (u *User) GetEmail() string {
	return u.Email
}
func (u User) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *User) GetPrivateKey() crypto.PrivateKey {
	return u.Key
}
func (u *User) SaveToDisk() error {
	b, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(config.Parameters.CertDirectory, certDirMode)
	userFile, _, _, _ := GetACMEFileNames(u.Domain, config.Parameters.CertDirectory)
	err = ioutil.WriteFile(userFile, b, certFileMode)
	if err != nil {
		return err
	}
	return nil
}

func GetUser() (*User, error) {
	u := User{}
	// do we have a user
	b, err := ioutil.ReadFile(config.Parameters.ACMEUserFile)
	if err == nil {
		// user exists
		err = json.Unmarshal(b, &u)
		if err != nil {
			return nil, err
		}
	} else {
		// create private Key
		privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, err
		}
		u.Key = privateKey
	}
	// get domain name
	commonName, err := node.GetDefaultDomainFromIP(config.Parameters.Hostname, config.Parameters.DefaultTlsDomainTmpl)
	if err != nil {
		return nil, err
	}
	u.Domain = commonName

	return &u, nil
}
