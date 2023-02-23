package election

import (
	"github.com/nknorg/nkn/v2/common"
	"math"
	"os"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

var c = &Config{
	Duration:                    5 * time.Second,
	MinVotingInterval:           200 * time.Millisecond,
	MaxVotingInterval:           2 * time.Second,
	ChangeVoteMinRelativeWeight: 0.5,
	ConsensusMinRelativeWeight:  2.0 / 3.0,
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func check(t *testing.T, f string, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s mismatch: got %v, want %v", f, got, want)
	}
}

func TestSingleElection(t *testing.T) {
	elc, err := NewElection(c)
	if err != nil {
		t.Fatal(err)
	}

	err = elc.SetInitialVote(common.MaxUint256)
	if err != nil {
		t.Fatal(err)
	}

	v1 := common.EmptyUint256
	v1[0] = 255

	err = elc.PrefillNeighborVotes([]interface{}{"n1", "n2", "n3"}, v1)
	if err != nil {
		t.Fatal(err)
	}

	success := elc.Start()
	if !success {
		t.Fatal("failed to start election")
	}

	txVoteChan := elc.GetTxVoteChan()

	for vote := range txVoteChan {
		v, ok := vote.(common.Uint256)
		if !ok {
			t.Fatal("unexpected vote type")
		}
		check(t, "vote", v, v1)
	}

	result, absWeight, relWeight, err := elc.GetResult()
	if err != nil {
		t.Fatal(err)
	}

	check(t, "result", result, v1)
	check(t, "absWeight", absWeight, uint32(4))
	check(t, "relWeight", almostEqual(float64(relWeight), 1), true)
}

func TestMultiElection(t *testing.T) {
	v1 := common.EmptyUint256
	v1[0] = 1

	elcList := make([]*Election, 20)

	var err error
	for i := 0; i < len(elcList); i++ {
		elcList[i], err = NewElection(c)
		if err != nil {
			t.Fatal(err)
		}
		var v common.Uint256
		if i < 14 {
			v = v1
		} else {
			v = common.MaxUint256
		}
		err = elcList[i].SetInitialVote(v)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < len(elcList); i++ {
		for j := 0; j < len(elcList); j++ {
			if i != j {
				err = elcList[i].PrefillNeighborVotes([]interface{}{strconv.Itoa(j)}, elcList[j].selfVote)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}

	for i := 0; i < len(elcList); i++ {
		go func(i int) {
			success := elcList[i].Start()
			if !success {
				t.Error("failed to start election:", i)
				return
			}
		}(i)
	}

	var wg sync.WaitGroup
	for i := 0; i < len(elcList); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			txVoteChan := elcList[i].GetTxVoteChan()
			for vote := range txVoteChan {
				for j, n := range elcList {
					if i != j {
						err = n.ReceiveVote(strconv.Itoa(i), vote)
						if err != nil {
							t.Error(err)
							return
						}
					}
				}
			}
		}(i)
	}

	wg.Wait()

	for _, n := range elcList {
		result, absWeight, relWeight, err := n.GetResult()
		if err != nil {
			t.Fatal(err)
		}
		check(t, "result", result, v1)
		check(t, "absWeight", absWeight, uint32(20))
		check(t, "relWeight", almostEqual(float64(relWeight), 1), true)
	}
}

func almostEqual(f1 float64, f2 float64) bool {
	return math.Abs(f1-f2) < 1e-9
}
