# Chord DHT in Go
Package chord provides library functions for the Chord DHT, as described by the original authors of the Chord protocol (https://pdos.csail.mit.edu/papers/ton:chord/paper-ton.pdf).

## Features

This library provides functions for:
* Creating a new DHT
* Joining nodes to an existing DHT
* Looking up DHT keys according to the Chord scalable lookup protocol
* Building applications to run on top of the Chord DHT

See the documentation for details.

## Dependencies

This package requires Protobufs. Details for installing protoc can be found here: https://github.com/google/protobuf

## Documentation
For a complete description of library functions, see https://godoc.org/github.com/cbocovic/chord
