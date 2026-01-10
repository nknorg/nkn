package common

import (
	"encoding/hex"
	"encoding/json"
	"slices"
	"testing"
)

// NKN address: NKNPkGps7i6yye6W2qSVsQUgiyqxWHEoTRby
// ToHexString(): 867ff0ca31905faec269949de37d27773ba7394e
const NKNADDRESS = "\"NKNPkGps7i6yye6W2qSVsQUgiyqxWHEoTRby\""
const NKNADDRESS_HEX = "867ff0ca31905faec269949de37d27773ba7394e"

func TestMarshalJSON(t *testing.T) {
	// Create instance of Uint160
	f := new(Uint160)

	dat, err := hex.DecodeString(NKNADDRESS_HEX)
	if err != nil {
		t.Fatal(err)
	}
	f.SetBytes(dat)

	// Expected value after Marshalling
	expected := []byte(NKNADDRESS)

	bytes, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !slices.Equal(bytes, expected) {
		t.Fatalf("Expected %v, got %v", expected, bytes)
	}

	bytes, err = json.Marshal(*f)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !slices.Equal(bytes, expected) {
		t.Fatalf("Expected %v, got %v", expected, bytes)
	}

	// TODO: If possible, create a scenario where ToAddress returns an error
	// and verify that MarshalJSON also returns this error.
}

func TestUnmarshalJSON(t *testing.T) {
	// Input for unmarshalling
	input := []byte(NKNADDRESS)

	f := new(Uint160)
	err := f.UnmarshalJSON(input)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify that f has been updated correctly
	expected, err := hex.DecodeString(NKNADDRESS_HEX)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.ToArray(), expected) {
		t.Fatalf("Expected %v, got %v", expected, f.ToArray())
	}

	// TODO: If possible, create a scenario where ToScriptHash returns an error
	// and verify that UnmarshalJSON also returns this error.
}
