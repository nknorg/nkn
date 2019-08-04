// Package ed25519 implements a verifiable random function using the Edwards form
// of Curve25519, SHA512 and the Elligator map.
//
//     E is Curve25519 (in Edwards coordinates), h is SHA512.
//     f is the elligator map (bytes->E) that covers half of E.
//     8 is the cofactor of E, the group order is 8*l for prime l.
//     Setup : the prover publicly commits to a public key (P : E)
//     H : names -> E
//         H(n) = f(h(n))^8
//     VRF : keys -> names -> vrfs
//         VRF_x(n) = h(n, H(n)^x))
//     Prove : keys -> names -> proofs
//         Prove_x(n) = tuple(c=h(n, g^r, H(n)^r), t=r-c*x, ii=H(n)^x)
//             where r = h(x, n) is used as a source of randomness
//     Check : E -> names -> vrfs -> proofs -> bool
//         Check(P, n, vrf, (c,t,ii)) = vrf == h(n, ii)
//                                     && c == h(n, g^t*P^c, H(n)^t*ii^c)
package vrf

import (
	"bytes"
	"crypto/rand"
	"crypto/sha512"
	"errors"
	"io"

	"github.com/nknorg/nkn/crypto/ed25519/edwards25519"
	"github.com/nknorg/nkn/crypto/ed25519/extra25519"

	"golang.org/x/crypto/ed25519"
)

const (
	PublicKeySize    = ed25519.PublicKeySize
	PrivateKeySize   = ed25519.PrivateKeySize
	Size             = 32
	intermediateSize = ed25519.PublicKeySize
	ProofSize        = 32 + 32 + intermediateSize
)

var (
	ErrGetPubKey = errors.New("[vrf] Couldn't get corresponding public-key from private-key")
)

type PrivateKey []byte
type PublicKey []byte

// GenerateKey creates a public/private key pair using rnd for randomness.
// If rnd is nil, crypto/rand is used.
func GenerateKey(rnd io.Reader) (sk PrivateKey, err error) {
	if rnd == nil {
		rnd = rand.Reader
	}
	_, skr, err := ed25519.GenerateKey(rnd)
	return PrivateKey(skr), err
}

// Public extracts the public VRF key from the underlying private-key
// and returns a boolean indicating if the operation was successful.
func (sk PrivateKey) Public() (PublicKey, bool) {
	pk, ok := ed25519.PrivateKey(sk).Public().(ed25519.PublicKey)
	return PublicKey(pk), ok
}

func (sk PrivateKey) expandSecret() (x, skhr *[32]byte) {
	x, skhr = new([32]byte), new([32]byte)
	skh := sha512.Sum512(sk[:32])
	copy(x[:], skh[:])
	copy(skhr[:], skh[32:])
	x[0] &= 248
	x[31] &= 127
	x[31] |= 64
	return
}

// Compute generates the vrf value for the byte slice m using the
// underlying private key sk.
func (sk PrivateKey) Compute(m []byte) []byte {
	x, _ := sk.expandSecret()
	var ii edwards25519.ExtendedGroupElement
	var iiB [32]byte
	edwards25519.GeScalarMult(&ii, x, hashToCurve(m))
	ii.ToBytes(&iiB)

	vrf := sha512.New()
	vrf.Write(iiB[:]) // const length: Size
	vrf.Write(m)
	return vrf.Sum(nil)[:32]
}

func hashToCurve(m []byte) *edwards25519.ExtendedGroupElement {
	// H(n) = (f(h(n))^8)
	hmbH := sha512.Sum512(m)
	var hmb [32]byte
	copy(hmb[:], hmbH[:])
	var hm edwards25519.ExtendedGroupElement
	extra25519.HashToEdwards(&hm, &hmb)
	edwards25519.GeDouble(&hm, &hm)
	edwards25519.GeDouble(&hm, &hm)
	edwards25519.GeDouble(&hm, &hm)
	return &hm
}

// Prove returns the vrf value and a proof such that
// Verify(m, vrf, proof) == true. The vrf value is the
// same as returned by Compute(m).
func (sk PrivateKey) Prove(m []byte, randSrc bool) (vrf, proof []byte) {
	x, skhr := sk.expandSecret()
	var sH, rH [64]byte
	var r, s, minusS, t, gB, grB, hrB, hxB, hB [32]byte
	var ii, gr, hr edwards25519.ExtendedGroupElement

	h := hashToCurve(m)
	h.ToBytes(&hB)
	edwards25519.GeScalarMult(&ii, x, h)
	ii.ToBytes(&hxB)

	// use hash of private-, public-key and msg as randomness source:
	hash := sha512.New()
	hash.Write(skhr[:])
	hash.Write(sk[32:]) // public key, as in ed25519
	hash.Write(m)
	if randSrc {
		b := make([]byte, len(rH))
		rand.Read(b)
		hash.Sum(b)
	}
	hash.Sum(rH[:0])
	hash.Reset()
	edwards25519.ScReduce(&r, &rH)

	edwards25519.GeScalarMultBase(&gr, &r)
	edwards25519.GeScalarMult(&hr, &r, h)
	gr.ToBytes(&grB)
	hr.ToBytes(&hrB)
	gB = edwards25519.BaseBytes

	// H2(g, h, g^x, h^x, g^r, h^r, m)
	hash.Write(gB[:])
	hash.Write(hB[:])
	hash.Write(sk[32:]) // ed25519 public-key
	hash.Write(hxB[:])
	hash.Write(grB[:])
	hash.Write(hrB[:])
	hash.Write(m)
	hash.Sum(sH[:0])
	hash.Reset()
	edwards25519.ScReduce(&s, &sH)

	edwards25519.ScNeg(&minusS, &s)
	edwards25519.ScMulAdd(&t, x, &minusS, &r)

	proof = make([]byte, ProofSize)
	copy(proof[:32], s[:])
	copy(proof[32:64], t[:])
	copy(proof[64:96], hxB[:])

	hash.Reset()
	hash.Write(hxB[:])
	hash.Write(m)
	vrf = hash.Sum(nil)[:Size]
	return
}

// Verify returns true iff vrf=Compute(m) for the sk that
// corresponds to pk.
func (pkBytes PublicKey) Verify(m, vrfBytes, proof []byte) bool {
	if len(proof) != ProofSize || len(vrfBytes) != Size || len(pkBytes) != PublicKeySize {
		return false
	}
	var pk, s, sRef, t, vrf, hxB, hB, gB, ABytes, BBytes [32]byte
	copy(vrf[:], vrfBytes)
	copy(pk[:], pkBytes[:])
	copy(s[:32], proof[:32])
	copy(t[:32], proof[32:64])
	copy(hxB[:], proof[64:96])

	hash := sha512.New()
	hash.Write(hxB[:]) // const length
	hash.Write(m)
	if !bytes.Equal(hash.Sum(nil)[:Size], vrf[:Size]) {
		return false
	}
	hash.Reset()

	var P, B, ii, iic edwards25519.ExtendedGroupElement
	var A, hmtP, iicP edwards25519.ProjectiveGroupElement
	if !P.FromBytesBaseGroup(&pk) {
		return false
	}
	if !ii.FromBytesBaseGroup(&hxB) {
		return false
	}
	edwards25519.GeDoubleScalarMultVartime(&A, &s, &P, &t)
	A.ToBytes(&ABytes)
	gB = edwards25519.BaseBytes

	h := hashToCurve(m) // h = H1(m)
	h.ToBytes(&hB)
	edwards25519.GeDoubleScalarMultVartime(&hmtP, &t, h, &[32]byte{})
	edwards25519.GeDoubleScalarMultVartime(&iicP, &s, &ii, &[32]byte{})
	iicP.ToExtended(&iic)
	hmtP.ToExtended(&B)
	edwards25519.GeAdd(&B, &B, &iic)
	B.ToBytes(&BBytes)

	var sH [64]byte
	// sRef = H2(g, h, g^x, v, g^t路G^s,H1(m)^t路v^s, m), with v=H1(m)^x=h^x
	hash.Write(gB[:])
	hash.Write(hB[:])
	hash.Write(pkBytes)
	hash.Write(hxB[:])
	hash.Write(ABytes[:]) // const length (g^t*G^s)
	hash.Write(BBytes[:]) // const length (H1(m)^t*v^s)
	hash.Write(m)
	hash.Sum(sH[:0])

	edwards25519.ScReduce(&sRef, &sH)
	return sRef == s
}
