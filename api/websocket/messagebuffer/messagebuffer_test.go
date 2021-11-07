package messagebuffer

import (
	"encoding/hex"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/nknorg/nkn/v2/pb"
)

func TestMessageQueue(t *testing.T) {
	mq := NewMessageQueue()
	msg1, msg2, msg3 := &pb.Relay{}, &pb.Relay{}, &pb.Relay{}
	_ = mq.append(msg1, 1)
	message := mq.append(msg2, 2)
	_ = mq.append(msg3, 3)
	if mq.length != 3 {
		t.Fatalf(`Get mq.lenght = %v, want %v`, mq.length, 3)
	}
	mq.delete(message)
	if mq.length != 2 {
		t.Fatalf(`Get mq.lenght = %v, want %v`, mq.length, 2)
	}
	sn1 := mq.pop().sequenceNum
	if mq.length != 1 {
		t.Fatalf(`Get mq.lenght = %v, want %v`, mq.length, 1)
	}
	sn2 := mq.pop().sequenceNum
	if mq.length != 0 {
		t.Fatalf(`Get mq.lenght = %v, want %v`, mq.length, 0)
	}
	if sn1 != 1 || sn2 != 3 {
		t.Fatalf(`Get sn1 = %v, want %v, Get sn2 = %v, want %v`, sn1, 1, sn2, 3)
	}
}

func TestMessageBufferExpiration(t *testing.T) {
	msg1, msg2, msg3 := &pb.Relay{MaxHoldingSeconds: 1}, &pb.Relay{MaxHoldingSeconds: 3}, &pb.Relay{MaxHoldingSeconds: 5}
	mb := NewMessageBuffer()
	mb.AddMessage([]byte("1"), msg1)
	mb.AddMessage([]byte("2"), msg2)
	mb.AddMessage([]byte("3"), msg3)
	if mb.length != 3 {
		t.Fatalf(`Get mB.lenght = %v, want %v`, mb.length, 3)
	}
	timer1 := time.After(time.Duration(2) * time.Second)
	timer2 := time.After(time.Duration(4) * time.Second)
	timer3 := time.After(time.Duration(6) * time.Second)

	for i := 0; i < 3; i++ {
		select {
		case <-timer1:
			_, ok := mb.buffer[hex.EncodeToString([]byte("1"))]
			if ok {
				t.Fatalf(`messasge queue of client 1 still exist after expiration time`)
			}
			_, ok = mb.buffer[hex.EncodeToString([]byte("2"))]
			if !ok {
				t.Fatalf(`messasge queue of client 2 does not exist before expiration time`)
			}
			_, ok = mb.buffer[hex.EncodeToString([]byte("3"))]
			if !ok {
				t.Fatalf(`messasge queue of client 3 does not exist before expiration time`)
			}
			if mb.length != 2 {
				t.Fatalf(`Get mb.lenght = %v, want %v`, mb.length, 2)
			}
		case <-timer2:
			_, ok := mb.buffer[hex.EncodeToString([]byte("1"))]
			if ok {
				t.Fatalf(`messasge queue of client 1 still exist after expiration time`)
			}
			_, ok = mb.buffer[hex.EncodeToString([]byte("2"))]
			if ok {
				t.Fatalf(`messasge queue of client 2 still exist after expiration time`)
			}
			_, ok = mb.buffer[hex.EncodeToString([]byte("3"))]
			if !ok {
				t.Fatalf(`messasge queue of client 3 does not exist before expiration time`)
			}
			if mb.length != 1 {
				t.Fatalf(`Get mb.lenght = %v, want %v`, mb.length, 1)
			}
		case <-timer3:
			_, ok := mb.buffer[hex.EncodeToString([]byte("1"))]
			if ok {
				t.Fatalf(`messasge queue of client 1 still exist after expiration time`)
			}
			_, ok = mb.buffer[hex.EncodeToString([]byte("2"))]
			if ok {
				t.Fatalf(`messasge queue of client 2 still exist after expiration time`)
			}
			_, ok = mb.buffer[hex.EncodeToString([]byte("3"))]
			if ok {
				t.Fatalf(`messasge queue of client 3 still exist after expiration time`)
			}
			if mb.length != 0 {
				t.Fatalf(`Get mb.lenght = %v, want %v`, mb.length, 0)
			}
		}
	}
}

func TestMessageBufferPopMesssages(t *testing.T) {
	msg1, msg2, msg3, msg4 := &pb.Relay{MaxHoldingSeconds: 3}, &pb.Relay{MaxHoldingSeconds: 3}, &pb.Relay{MaxHoldingSeconds: 3}, &pb.Relay{MaxHoldingSeconds: 3}
	mb := NewMessageBuffer()
	mb.AddMessage([]byte("1"), msg1)
	mb.AddMessage([]byte("2"), msg2)
	mb.AddMessage([]byte("3"), msg3)
	mb.AddMessage([]byte("2"), msg4)
	if mb.length != 4 {
		t.Fatalf(`Get mB.lenght = %v, want %v`, mb.length, 4)
	}
	timer1 := time.After(time.Duration(1) * time.Second)
	timer2 := time.After(time.Duration(4) * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-timer1:
			_ = mb.PopMessages([]byte("2"))
			_, ok := mb.buffer[hex.EncodeToString([]byte("1"))]
			if !ok {
				t.Fatalf(`messasge queue of client 1 does not exist before expiration time`)
			}
			_, ok = mb.buffer[hex.EncodeToString([]byte("2"))]
			if ok {
				t.Fatalf(`messasge queue of client 2 still exist after pop`)
			}
			_, ok = mb.buffer[hex.EncodeToString([]byte("3"))]
			if !ok {
				t.Fatalf(`messasge queue of client 3 does not exist before expiration time`)
			}
			if mb.length != 2 {
				t.Fatalf(`Get mb.lenght = %v, want %v`, mb.length, 2)
			}
		case <-timer2:
			_, ok := mb.buffer[hex.EncodeToString([]byte("1"))]
			if ok {
				t.Fatalf(`messasge queue of client 1 still exist after expiration time`)
			}
			_, ok = mb.buffer[hex.EncodeToString([]byte("2"))]
			if ok {
				t.Fatalf(`messasge queue of client 2 still exist after pop`)
			}
			_, ok = mb.buffer[hex.EncodeToString([]byte("3"))]
			if ok {
				t.Fatalf(`messasge queue of client 3 still exist after expiration time`)
			}
			if mb.length != 0 {
				t.Fatalf(`Get mb.lenght = %v, want %v`, mb.length, 0)
			}
		}
	}
}

func TestMessageBufferAutoRemove(t *testing.T) {
	msg1, msg2, msg3, msg4 := &pb.Relay{MaxHoldingSeconds: 3}, &pb.Relay{MaxHoldingSeconds: 3}, &pb.Relay{MaxHoldingSeconds: 3}, &pb.Relay{MaxHoldingSeconds: 3}
	mb := NewMessageBuffer()
	mb.capacity = 3
	mb.AddMessage([]byte("1"), msg1)
	mb.AddMessage([]byte("2"), msg2)
	mb.AddMessage([]byte("3"), msg3)
	if mb.length != 3 {
		t.Fatalf(`Get mB.lenght = %v, want %v`, mb.length, 4)
	}
	mb.AddMessage([]byte("2"), msg4)
	if mb.length != 3 {
		t.Fatalf(`Get mB.lenght = %v, want %v`, mb.length, 4)
	}
	_, ok := mb.buffer[hex.EncodeToString([]byte("1"))]
	if ok {
		t.Fatalf(`messasge queue of client 1 still exists after removed`)
	}
}

func TestMessageBufferConcurrencyControl(t *testing.T) {
	mb := NewMessageBuffer()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			msg := &pb.Relay{MaxHoldingSeconds: 3}
			mb.AddMessage([]byte(strconv.Itoa(rand.Int())), msg)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			msg := &pb.Relay{MaxHoldingSeconds: 6}
			mb.AddMessage([]byte(strconv.Itoa(rand.Int())), msg)
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			msg := &pb.Relay{MaxHoldingSeconds: 9}
			mb.AddMessage([]byte(strconv.Itoa(rand.Int())), msg)
		}
	}()
	wg.Wait()
	if mb.length != 1500 {
		t.Fatalf(`Get mB.lenght = %v, want %v`, mb.length, 1500)
	}
	timer1 := time.After(time.Duration(4) * time.Second)
	timer2 := time.After(time.Duration(7) * time.Second)
	timer3 := time.After(time.Duration(10) * time.Second)
	for i := 0; i < 3; i++ {
		select {
		case <-timer1:
			if mb.length != 1000 {
				t.Fatalf(`Get mB.lenght = %v, want %v`, mb.length, 1000)
			}
		case <-timer2:
			if mb.length != 500 {
				t.Fatalf(`Get mB.lenght = %v, want %v`, mb.length, 500)
			}
		case <-timer3:
			if mb.length != 0 {
				t.Fatalf(`Get mB.lenght = %v, want %v`, mb.length, 0)
			}
		}
	}
}
