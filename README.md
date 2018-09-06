[![Go Report Card](https://nkn.org/badge/nkn.svg)](https://goreportcard.com/report/github.com/nknorg/nkn)
[![Build Status](https://travis-ci.org/nknorg/nkn.svg?branch=master)](https://travis-ci.org/nknorg/nkn)

[![NKN](https://github.com/nknorg/nkn/wiki/img/nkn_logo.png)](https://nkn.org)

# NKN: a Scalable Self-Evolving and Self-Incentivized Decentralized Network

> NKN, short for New Kind of Network, is a project aiming to rebuild the
> Internet that will be truly open, decentralized, dynamic, safe, shared and
> owned by the community.

Official website: [https://nkn.org/](https://nkn.org/)

## Introduction

The core of the NKN network consists of many connected nodes distributed
globally. Every node is only connected to and aware of a few other nodes called
neighbors. Packets can be transmitted from any node to any other node in an
efficient and verifiable route. Data can be sent to any clients without public
or static IP address using their permanent NKN address with end-to-end
encryption.

The relay workload can be verified using our Proof of Relay (PoR) algorithm. A
small and fixed portion of the packets will be randomly selected as proof. The
random selection can be verified and cannot be predicted or controlled. Proof
will be sent to other nodes for payment and rewards.

A node in our network is both relayer and consensus participant. Consensus among
massive nodes can be reached efficiently by only communicating with neighbors
using our consensus algorithm based on Cellular Automata. Consensus is reached
for every block to prevent fork.

More details can be found in [our wiki](https://github.com/nknorg/nkn/wiki).

## Technical Highlights

* Transmit any data to any node/client without any centralized server. [Related tech design doc](https://github.com/nknorg/nkn/wiki/Tech-Design-Doc%3A-Distributed-Data-Transmission-Network-%28DDTN%29)
* Proof-of-Relay, a useful proof of work: mining is relaying data. [Related tech design doc](https://github.com/nknorg/nkn/wiki/Tech-Design-Doc%3A-Proof-of-Relay-%28PoR%29)
* Extremely scalable consensus algorithm (millions or even billions of nodes). [Related tech design doc](https://github.com/nknorg/nkn/wiki/Tech-Design-Doc%3A-Consensus-and-Blockchain)
* Strong consistency rather than eventual consistency. [Related tech design doc](https://github.com/nknorg/nkn/wiki/Tech-Design-Doc%3A-Consensus-and-Blockchain)
* Dynamic, large-scale network. [Related tech design doc](https://github.com/nknorg/nkn/wiki/Tech-Design-Doc%3A-Distributed-Data-Transmission-Network-%28DDTN%29)
* Verifiable topology and routes. [Related tech design doc](https://github.com/nknorg/nkn/wiki/Tech-Design-Doc%3A-Relay-Path-Validation)
* Secure address scheme with public key embedded. [Related tech design doc](https://github.com/nknorg/nkn/wiki/Tech-Design-Doc%3A-NKN-Address-Scheme)

## Deployment

**Note: this repository is in the early development stage and may not
have all functions working properly. It should be used only for testing
now.**

**Q:** I want to run this node, but have no idea about programming or temrinal.
What should I do?

**A:** Easiest for you will be to follow [docker instructions](#building-using-docker) below. Docker will take care of quite a lot of things for you.
If you are asked to run or issue command (usually formatted like this:)
```shell
cd change/active/directory/to/this/one
```
open a terminal (or cmd on windows - start -> run/search -> cmd.exe) and write the command there.


### Building from source


To build from source, you need a properly configured Go environment (Go 1.8+,
$GOROOT and $GOPATH set up, see [Go Official Installation
Documentation](https://golang.org/doc/install) for more details).

Create directory $GOPATH/src/github.com/nknorg/ if not exists

In directory $GOPATH/src/github.com/nknorg/ clone the repository

```shell
$ git clone https://github.com/nknorg/nkn.git
```

Install package management tool `glide` if it's not installed on your system.

```shell
$ cd nkn
$ make glide
```

Download dependencies for building
```shell
$ make vendor
```

Build the source code with make

```shell
$ make
```

After building is successful, you should see two executables:

* `nknd`: the nkn node program
* `nknc`: command line tool for nkn node control

Now you can see [configuration](#configuration) for how to configure and run a
node.

You can also build binaries for other architectures by executing `make all`. The
resulting binaries are stored in `build` directory.

### Building using docker

*Prerequirement*: Have working docker software installed. For help with that
visit [official docker
docs](https://docs.docker.com/install/#supported-platforms)

Build and tag Docker image

```shell
$ docker build -t nkn .
```

When starting the container, a directory with configuration files containing
`config.json` (see [configuration](#configuration)) and `wallet.dat` (if exists)
should be mapped to `/nkn` directory in the container. This directory will also
be used for wallet, block data and logs storage.

### Configuration

When starting a node, it will read the configurations from `config.json`. We
provide two sample `config.json`:

* `config.testnet.json`: join the testnet
* `config.local.json`: create and join a private chain on your localhost

You can copy the one you want to `config.json` or write your own.

Before starting the node, you need to create a new wallet first:

``` shell
$ ./nknc wallet -c
Password:
Re-enter Password:
Address                            Public Key
-------                            ----------
NjCWGM1EfJeopJopSQGC6aLEkuug5GiwLM 03d45f701e7e330e1fd1c7cce09ffb95f7b1870e5c429ad8e8c950ddb879093f52
```

The last line of the output is the public key of this wallet, and the second
last line is the wallet address. A wallet address always starts with `N`.

Wallet information will be saved at `wallet.dat` and it's encrypted with the
password you provided when creating the wallet. So please make sure you pick a
strong password and remember it!

Now you can [join the testnet](#join-the-testnet) or [create a private
chain](https://github.com/nknorg/nkn/wiki/Create-a-Private-Chain).

### Join the TestNet

**[IMPORTANT] At the current stage, in order to join the Testnet, you need to
have a public IP address, or set up [port forwarding](#port-forwarding) on your
router properly so that other people can establish connection to you.**

If you have done the previous steps correctly (`config.json`, create wallet,
public IP or port forwarding), joining the testnet is as simple as running:

```shell
$ ./nknd
```

If you are using Docker then you should run the following command instead:

```shell
$ docker run -p 30000-30003:30000-30003 -v $PWD:/nkn --name nkn -it nkn nknd
```

If everything goes well, you should be part of our TestNet now! You can query
your wallet balance (which includes the Testnet token you've mined) by:

```shell
$ ./nknc wallet -l balance
```

or if you are using Docker:

```shell
$ docker exec -it nkn nknc wallet -l balance
```

**Note that Testnet token is for testing purpose only, and may be cleared at any
time when Testnet resets.**

If anything goes wrong, you may want to check if any of the previous steps went
wrong. If the problem still persists, [create an
issue](https://github.com/nknorg/nkn/issues/new) or ask us in our [Discord
group](#community).

### Port forwarding

Most likely your node is behind a router and does not have a public IP address.
In such case, you **have to** setup port forwarding on your router for **all**
ports specified in `config.json`, otherwise other nodes cannot establish
connections to you.

When setting up port forwarding, public port needs to be the same as private
port mapped to your node. For example, you should map port 30001 on your
router's public IP address to port 30001 on your node's internal IP address.

The specific steps to setup port forwarding depends on your router. But in
general, you need to log in to the admin interface of your router (typically in
a web browser), then navigate to the port forwarding section, and create several
mappings, one for each port. One of the easiest way to find out how to setup
port forwarding on your router is to search "how to setup port forwarding" +
your router model or name online.

## Contributing

**Can I submit a bug, suggestion or feature request?**

Yes. Please [open an issue](https://github.com/nknorg/nkn/issues/new) for that.

**Can I contribute patches to NKN project?**

Yes, we appreciate your help! To make contributions, please fork the repo, push
your changes to the forked repo with signed-off commits, and open a pull request
here.

Please follow our [Golang Style Guide](https://github.com/nknorg/nkn/wiki/NKN-Golang-Style-Guide)
for coding style.

Please sign off your commit. This means adding a line "Signed-off-by: Name
<email>" at the end of each commit, indicating that you wrote the code and have
the right to pass it on as an open source patch. This can be done automatically
by adding -s when committing:

```shell
git commit -s
```

## Community

* [Discord](https://discord.gg/c7mTynX)
* [Telegram](https://t.me/nknorg)
* [Reddit](https://www.reddit.com/r/nknblockchain/)
* [Twitter](https://twitter.com/NKN_ORG)
