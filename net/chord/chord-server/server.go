package main

import (
	"flag"
	"fmt"
	"github.com/cbocovic/chord"
	"io"
)

func main() {

	//set up flags
	addressPtr := flag.String("addr", "127.0.0.1:8888", "the port you will listen on for incomming messages")
	joinPtr := flag.String("join", "", "an address of a server in the Chord network to join to")

	flag.Parse()
	me := new(chord.ChordNode)

	//join node to network or start a new network
	if *joinPtr == "" {
		me = chord.Create(*addressPtr)
	} else {
		me = chord.Join(*addressPtr, *joinPtr)
	}
	fmt.Printf("My address is: %s.\n", *addressPtr)
	//block until receive input
Loop:
	for {
		var cmd string
		_, err := fmt.Scan(&cmd)
		switch {
		case cmd == "print":
			//print out successor and predecessor
			fmt.Printf("%s", me.Info())
		case cmd == "fingers":
			//print out finger table
			fmt.Printf("%s", me.ShowFingers())
		case cmd == "succ":
			//print out successor list
			fmt.Printf("%s", me.ShowSucc())
		case err == io.EOF:
			break Loop
		}

	}
	me.Finalize()

}
