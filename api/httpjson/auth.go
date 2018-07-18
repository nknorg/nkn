package httpjson

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"strings"
)

type RPCAuthType byte

const (
	UsernamePassword RPCAuthType = 0x01
)

type Authorization interface {
	GetAuthType() RPCAuthType
	CheckAuth(s *RPCServer, r *http.Request) bool
}

type UserPassword struct {
	Username     string
	PasswordHash string
}

func NewUserPassword(username string, passwordHash string) Authorization {
	return &UserPassword{
		Username:     username,
		PasswordHash: passwordHash,
	}
}

func (up *UserPassword) GetAuthType() RPCAuthType {
	return UsernamePassword
}

func (up *UserPassword) CheckAuth(s *RPCServer, r *http.Request) bool {
	target := up.Username + ":" + up.PasswordHash
	targetHash := sha256.Sum256([]byte(target))

	metadata := r.Header["Authorization"]
	if len(metadata) <= 0 {
		return false
	}
	metadataSlice := strings.Split(metadata[0], ":")
	if len(metadataSlice) != 2 {
		return false
	}

	username := metadataSlice[0]
	password := metadataSlice[1]

	pwdHash := sha256.Sum256([]byte(password))
	pwdHash2 := sha256.Sum256([]byte(pwdHash[:]))

	login := username + ":" + string(pwdHash2[:])
	loginHash := sha256.Sum256([]byte(login))

	cmp := subtle.ConstantTimeCompare(loginHash[:], targetHash[:])
	if cmp == 1 {
		return true
	}

	return false

}
