package common

import (
	"testing"
)

func TestPrque(t *testing.T) {

	pq := NewPrque()

	pq.Push("banana", 1)
	pq.Push("apple", 2)
	pq.Push("orange", 3)

	o, _ := pq.Pop()
	if o != "orange" {
		t.Errorf("orange priority not poped")
	}

	a, _ := pq.Pop()
	if a != "apple" {
		t.Errorf("apple priority not poped")
	}

	b, _ := pq.Pop()
	if b != "banana" {
		t.Errorf("banana priority not poped")
	}
}
