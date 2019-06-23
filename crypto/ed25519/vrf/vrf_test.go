package vrf

import (
	"bytes"
	"fmt"
	"testing"
)

func TestHonestComplete(t *testing.T) {
	sk, err := GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	pk, _ := sk.Public()
	alice := []byte("alice")
	aliceVRF := sk.Compute(alice)
	aliceVRFFromProof, aliceProof := sk.Prove(alice, false)

	fmt.Printf("pk:           %X\n", pk)
	fmt.Printf("sk:           %X\n", sk)
	fmt.Printf("alice(bytes): %X\n", alice)
	fmt.Printf("aliceVRF:     %X\n", aliceVRF)
	fmt.Printf("aliceProof:   %X\n", aliceProof)
	fmt.Printf("aliceVRFFromProof:   %X\n", aliceVRFFromProof)

	if !pk.Verify(alice, aliceVRF, aliceProof) {
		t.Error("Gen -> Compute -> Prove -> Verify -> FALSE")
	}
	if !bytes.Equal(aliceVRF, aliceVRFFromProof) {
		t.Error("Compute != Prove")
	}
}

func TestConvertPrivateKeyToPublicKey(t *testing.T) {
	sk, err := GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	pk, ok := sk.Public()
	if !ok {
		t.Fatal("Couldn't obtain public key.")
	}
	if !bytes.Equal(sk[32:], pk) {
		t.Fatal("Raw byte respresentation doesn't match public key.")
	}
}

func TestFlipBitForgery(t *testing.T) {
	sk, err := GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	pk, _ := sk.Public()
	alice := []byte("alice")
	for i := 0; i < 32; i++ {
		for j := uint(0); j < 8; j++ {
			aliceVRF := sk.Compute(alice)
			aliceVRF[i] ^= 1 << j
			_, aliceProof := sk.Prove(alice, false)
			if pk.Verify(alice, aliceVRF, aliceProof) {
				t.Fatalf("forged by using aliceVRF[%d]^=%d:\n (sk=%x)", i, j, sk)
			}
		}
	}
}

func sampleVectorTest(pk PublicKey, aliceVRF, aliceProof []byte, t *testing.T) {
	alice := []byte{97, 108, 105, 99, 101}

	// Positive test case
	if !pk.Verify(alice, aliceVRF, aliceProof) {
		t.Error("TestSampleVectors HonestVector Failed")
	}

	// Negative test cases - try increment the first byte of every vector
	pk[0]++
	if pk.Verify(alice, aliceVRF, aliceProof) {
		t.Error("TestSampleVectors ForgedVector (pk modified) Passed")
	}
	pk[0]--

	alice[0]++
	if pk.Verify(alice, aliceVRF, aliceProof) {
		t.Error("TestSampleVectors ForgedVector (alice modified) Passed")
	}
	alice[0]--

	aliceVRF[0]++
	if pk.Verify(alice, aliceVRF, aliceProof) {
		t.Error("TestSampleVectors ForgedVector (aliceVRF modified) Passed")
	}
	aliceVRF[0]--

	aliceProof[0]++
	if pk.Verify(alice, aliceVRF, aliceProof) {
		t.Error("TestSampleVectors ForgedVector (aliceProof modified) Passed")
	}
	aliceProof[0]--
}

func TestSampleVectorSets(t *testing.T) {
	t.Skip("TODO: generate new test vectors or remove test")
	var aliceVRF, aliceProof []byte
	var pk []byte

	// Following sets of test vectors are collected from TestHonestComplete(),
	// and are used for testing the JS implementation of vrf.verify()
	// Reference: https://github.com/yahoo/end-to-end/pull/58

	pk = []byte{194, 191, 96, 139, 106, 249, 24, 253, 198, 131, 88, 169, 100, 231, 7, 211, 70, 171, 171, 207, 24, 30, 150, 114, 77, 124, 240, 123, 191, 14, 29, 111}
	aliceVRF = []byte{68, 98, 55, 78, 153, 189, 11, 15, 8, 238, 132, 5, 53, 28, 232, 22, 222, 98, 21, 139, 89, 67, 111, 197, 213, 75, 86, 226, 178, 71, 245, 159}
	aliceProof = []byte{49, 128, 4, 253, 103, 241, 164, 51, 21, 45, 168, 55, 18, 103, 22, 233, 245, 136, 242, 238, 113, 218, 160, 122, 129, 89, 72, 103, 250, 222, 3, 3, 239, 235, 93, 98, 173, 115, 168, 24, 222, 165, 186, 224, 138, 76, 201, 237, 130, 201, 47, 18, 191, 24, 61, 80, 113, 139, 246, 233, 23, 94, 177, 12, 193, 106, 38, 172, 66, 192, 22, 188, 177, 14, 144, 100, 38, 179, 96, 70, 55, 157, 80, 139, 145, 62, 94, 195, 181, 224, 183, 42, 64, 66, 145, 162}
	sampleVectorTest(pk, aliceVRF, aliceProof, t)

	pk = []byte{133, 36, 180, 21, 60, 103, 35, 92, 204, 245, 236, 174, 242, 50, 212, 69, 124, 230, 1, 106, 94, 95, 201, 55, 208, 252, 195, 13, 12, 96, 87, 170}
	aliceVRF = []byte{35, 127, 188, 177, 246, 242, 213, 46, 16, 72, 1, 196, 69, 181, 160, 204, 69, 230, 17, 147, 251, 207, 203, 184, 154, 122, 118, 10, 144, 76, 229, 234}
	aliceProof = []byte{253, 33, 80, 241, 250, 172, 198, 28, 16, 171, 161, 194, 110, 175, 158, 233, 250, 89, 35, 174, 221, 101, 98, 136, 32, 191, 82, 127, 92, 208, 199, 10, 123, 46, 70, 95, 56, 102, 63, 137, 53, 160, 128, 216, 134, 152, 87, 58, 19, 244, 167, 108, 144, 13, 97, 232, 207, 75, 107, 57, 193, 124, 231, 5, 242, 122, 182, 247, 155, 187, 86, 165, 114, 46, 188, 52, 21, 121, 238, 100, 85, 32, 119, 116, 250, 208, 32, 60, 145, 53, 145, 76, 84, 153, 185, 28}
	sampleVectorTest(pk, aliceVRF, aliceProof, t)

	pk = []byte{85, 126, 176, 228, 114, 43, 110, 223, 111, 129, 204, 38, 215, 110, 165, 148, 223, 232, 79, 254, 150, 107, 61, 29, 216, 14, 238, 104, 55, 163, 121, 185}
	aliceVRF = []byte{171, 240, 42, 215, 128, 5, 247, 64, 164, 154, 198, 231, 6, 174, 207, 10, 95, 231, 117, 189, 88, 103, 72, 229, 43, 218, 184, 162, 44, 183, 196, 159}
	aliceProof = []byte{99, 103, 243, 119, 251, 30, 21, 57, 69, 162, 192, 80, 7, 49, 244, 136, 13, 252, 150, 165, 215, 181, 55, 203, 141, 124, 197, 36, 20, 183, 239, 14, 238, 213, 240, 96, 181, 187, 24, 137, 152, 152, 38, 186, 80, 141, 72, 15, 209, 178, 60, 205, 22, 31, 101, 185, 225, 159, 22, 118, 84, 179, 95, 0, 124, 140, 237, 187, 8, 77, 233, 213, 207, 211, 251, 153, 71, 112, 61, 89, 53, 26, 195, 167, 254, 73, 218, 135, 145, 89, 12, 4, 16, 255, 63, 89}
	sampleVectorTest(pk, aliceVRF, aliceProof, t)

}

func BenchmarkHashToGE(b *testing.B) {
	alice := []byte("alice")
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		hashToCurve(alice)
	}
}

func BenchmarkCompute(b *testing.B) {
	sk, err := GenerateKey(nil)
	if err != nil {
		b.Fatal(err)
	}
	alice := []byte("alice")
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sk.Compute(alice)
	}
}

func BenchmarkProve(b *testing.B) {
	sk, err := GenerateKey(nil)
	if err != nil {
		b.Fatal(err)
	}
	alice := []byte("alice")
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		sk.Prove(alice, false)
	}
}

func BenchmarkVerify(b *testing.B) {
	sk, err := GenerateKey(nil)
	if err != nil {
		b.Fatal(err)
	}
	alice := []byte("alice")
	aliceVRF := sk.Compute(alice)
	_, aliceProof := sk.Prove(alice, false)
	pk, _ := sk.Public()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		pk.Verify(alice, aliceVRF, aliceProof)
	}
}
