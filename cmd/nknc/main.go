package main

import (
	"log"

	cmd "github.com/robertsarosi/rsvpn/v2/cmd/nknc/commands"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Fatalf("Panic: %+v", r)
		}
	}()

	cmd.Execute()
}
