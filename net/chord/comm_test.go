package chord

import (
	"runtime"
	"testing"
	"time"
)


func TestNodeStart(t *testing.T) {
	ml := InitMLTransport()
	conf := DefaultConfig("test")
	conf.StabilizeMin = time.Duration(15 * time.Millisecond)
	conf.StabilizeMax = time.Duration(45 * time.Millisecond)
	r, err := Create(conf, ml)
	if err != nil {
		t.Fatalf("unexpected err. %s", err)
	}

	vn := makeVnode()
	vn.init()

}

func TestNodeJoin(t *testing.T) {
	ml := InitMLTransport()
	conf2 := DefaultConfig("test")
	conf2.StabilizeMin = time.Duration(15 * time.Millisecond)
	conf2.StabilizeMax = time.Duration(45 * time.Millisecond)
	conf2.Hostname = "test2"
	r2, err := Join(conf2, ml, "test")
	if err != nil {
		t.Fatalf("failed to join node! Got %s", err)
	}

	vn := makeVnode()
	vn.init()
}
