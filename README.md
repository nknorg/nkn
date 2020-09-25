# NKN Full Node

[![GitHub license](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE) [![Go Report Card](https://goreportcard.com/badge/github.com/nknorg/nkn)](https://goreportcard.com/report/github.com/nknorg/nkn) [![Build Status](https://travis-ci.org/nknorg/nkn.svg?branch=master)](https://travis-ci.org/nknorg/nkn) [![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#contributing)

[![NKN](https://github.com/nknorg/nkn/wiki/img/nkn_logo.png)](https://nkn.org)

Official Go implementation of NKN full node.

> NKN, short for New Kind of Network, is a project aiming to rebuild the
> Internet that will be truly open, decentralized, dynamic, safe, shared and
> owned by the community.

Official website: [https://nkn.org/](https://nkn.org/)

Note: This is the official **full node** implementation of the NKN protocol,
which relays data for clients and earn mining rewards. For **client**
implementation which can send and receive data, please refer to:

* [nkn-sdk-go](https://github.com/nknorg/nkn-sdk-go)
* [nkn-sdk-js](https://github.com/nknorg/nkn-sdk-js)
* [nkn-java-sdk](https://github.com/nknorg/nkn-java-sdk)

## Introduction

The core of the NKN network consists of many connected nodes distributed
globally. Every node is only connected to and aware of a few other nodes called
neighbors. Packets can be transmitted from any node to any other node in an
efficient and verifiable route. Data can be sent to any clients without public
or static IP address using their permanent NKN address with end-to-end
encryption. The network stack of NKN network is open source at another repo
called [nnet](https://github.com/nknorg/nnet) that can be used to build other
decentralized/distributed systems.

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

* Transmit any data to any node/client without any centralized server.
* Proof-of-Relay, a useful proof of work: mining is relaying data.
* Extremely scalable consensus algorithm (billions of nodes within seconds).
* Strong consistency rather than eventual consistency.
* Dynamic, large-scale network.
* Verifiable topology and routes.
* Secure address scheme with public key embedded.

### Use pre-built binaries

You just need to download and decompress the correct version matching your OS
and architecture from [github releases](https://github.com/nknorg/nkn/releases).

Now you can jump to [configuration](#configuration) for how to configure and run
a node.

### Use pre-built Docker image

*Prerequirement*: Have working docker software installed. For help with that
visit [official docker
docs](https://docs.docker.com/install/#supported-platforms)

We host latest Docker image (the same as you [build with
docker](#building-using-docker)) on our official Docker Hub account. You can get
it by

```shell
$ docker pull nknorg/nkn
```

Now you can jump to [configuration](#configuration) for how to configure and run
a node.

### Building using Docker

*Prerequirement*: Have working docker software installed. For help with that
visit [official docker
docs](https://docs.docker.com/install/#supported-platforms)

Build and tag Docker image

```shell
$ docker build -f docker/Dockerfile -t nknorg/nkn .
```

This command should be run once every time you update the code base.

### Building from source

To build from source, you need a properly configured Go environment (Go 1.13+,
see [Go Official Installation Documentation](https://golang.org/doc/install) for
more details).

```shell
$ git clone https://github.com/nknorg/nkn.git
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

### Configuration

When starting a NKN node (i.e. running `nknd`), it will reads a few configurable
files: `config.json` for configuration, `wallet.json` for wallet, and `certs/*`
for certificates. Additionally, it will read directory `web` for web GUI
interface static assets. By default `nknd` assumes they are located in the
current working directory.

For Docker, a directory containing `config.json`, `wallet.json` (if exists) and
`certs/` should be mapped to `/nkn/data` directory in the container. If not
provided, the default config and certs will be copied to `/nkn/data/`, and a
wallet and random password will be generated and saved to `/nkn/data/` on nknd
launch.

The path of config file, wallet file, database directory and log directory can
be specified by passing arguments to `nknd` or in `config.json`, run `nknd
--help` for more information.

#### `config.json`:

We provide a few sample `config.json`:

* `config.mainnet.json`: join the mainnet
* `config.testnet.json`: join the testnet
* `config.local.json`: create and join a private chain on your localhost

You can copy the one you want to `config.json` or write your own.

For convenience, we ship a copy of `config.mainnet.json` in release version (as
`default.json`) and in docker image (under `/nkn/`). The docker container will
copy this default one to `/nkn/data/config.json` if not exists on nknd launch.

#### `wallet.json`:

Before starting the node, you need to create a new wallet first. Wallet
information will be saved at `wallet.json` and it's encrypted with the password
you provided when creating the wallet. So please make sure you pick a strong
password and remember it!

``` shell
$ ./nknc wallet -c
Password:
Re-enter Password:
Address                                Public Key
-------                                ----------
NKNRQxosmUixL8bvLAS5G79m1XNx3YqPsFPW   35db285ea2f91499164cd3e19203ab5e525df6216d1eba3ac6bcef00503407ce
```

**[IMPORTANT] Each node needs to use a unique wallet. If you use share wallet
among multiple nodes, only one of them will be able to join the network!**

If you are using Docker, it should be `docker run -it -v ${PWD}:/nkn/data
nknorg/nkn nknc wallet -c` instead, assuming you want to store the `wallet.json`
in your current working directory. If you want it to be saved to another
directory, you need to change `${PWD}` to that directory.

The docker container will create a wallet saved to `/nkn/data/wallet.json` and a
random password saved to `/nkn/data/wallet.pswd` if not exists on nknd launch.

#### `certs/`

`nknd` uses Let's Encrypt to apply and renew TLS certificate and put in into
`cert/` directory.

#### Data and Logs

After `nknd` starts, it will creates two directories: `ChainDB` to store
blockchain data, and `Log` to store logs. By default `nknd` will creates these
directories in the current working directory, but it can be changed by passing
`--chaindb` and `--log` arguments to `nknd` or specify in config.json.

Now you can [join the mainnet](#join-the-mainnet), [join the
testnet](#join-the-testnet) or [create a private
chain](https://github.com/nknorg/nkn/wiki/Create-a-Private-Chain).

### Join the MainNet

**[IMPORTANT] In order to join the MainNet, you need to have a public IP
address, or set up [port forwarding](#port-forwarding) on your router properly
so that other people can establish connection to you.**

If you have done the previous steps correctly (`config.json`, create wallet,
public IP or port forwarding), joining the MainNet is as simple as running:

```shell
$ ./nknd
```

If you are using Docker then you should run the following command instead:

```shell
$ docker run -p 30001-30005:30001-30005 -v ${PWD}:/nkn/data --name nkn --rm -it nknorg/nkn
```

If you would like to enable web GUI interface from outside of the container, you
need to replace `-p 30001-30005:30001-30005` with `-p 30000-30005:30000-30005`.

If you get an error saying `docker: Error response from daemon: Conflict. The
container name "/nkn" is already in use by container ...`, you should run
`docker rm nkn` first to remove the old container.

If everything goes well, you should be part of the MainNet after a few minutes!
You can query your wallet balance (which includes the NKN token you've mined)
by:

```shell
$ ./nknc wallet -l balance
```

or if you are using Docker:

```shell
$ docker exec -it nkn nknc wallet -l balance
```

If there is a problem, you may want to check if any of the previous steps went
wrong. If the problem still persists, [create an
issue](https://github.com/nknorg/nkn/issues/new) or ask us in our [Discord
group](#community).

### [Recommended] Using BeneficiaryAddr

By default, token mined by your node will be sent to the wallet your node is
using, which is NOT as safe as you might think. The recommended way is to use
another cold wallet (that is saved and backed up well) to store your token. You
can use your code wallet address as `BeneficiaryAddr` in `config.json` such that
token mined by your node will be sent directly to that beneficiary address. This
is safer and more convenient because: 1. even if your node is hacked, or your
node wallet is leaked, you will not lose any token; 2. if you run multiple
nodes, it's the only way that all their mining rewards will go to the same
address.

### NAT traversal and port forwarding

Most likely your node is behind a router and does not have a public IP address.
By default, `nknd` will try to detect if your router supports UPnP or NAT-PMP
protocol, and if success, it will try to set up port forwarding automatically.
You can add `--no-nat` flag when starting nknd OR add `"NAT": false` in
`config.json` to disable automatic port forwarding. If your router does not
support such protocol, you **have to** setup port forwarding on your router for
port 30001 as well as **all** other ports specified in `config.json`
(30001-30005 by default), otherwise other nodes cannot establish connections to
you and you will **NOT** be able to mine token even though your node can still
run and sync blocks.

When setting up port forwarding, public port needs to be the same as private
port mapped to your node. For example, you should map port 30001 on your
router's public IP address to port 30001 on your node's internal IP address.

The specific steps to setup port forwarding depends on your router. But in
general, you need to log in to the admin interface of your router (typically in
a web browser), then navigate to the port forwarding section, and create several
mappings, one for each port. One of the easiest way to find out how to setup
port forwarding on your router is to search "how to setup port forwarding" +
your router model or name online.

### Join the TestNet

Joining the TestNet is the same as joining MainNet, except for using
`config.testnet.json` as your config file instead of `config.mainnet.json`. Note
that TestNet token is for testing purpose only (thus do not have value), and may
be cleared at any time when TestNet upgrades.

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

* [Forum](https://forum.nkn.org/)
* [Discord](https://discord.gg/c7mTynX)
* [Telegram](https://t.me/nknorg)
* [Reddit](https://www.reddit.com/r/nknblockchain/)
* [Twitter](https://twitter.com/NKN_ORG)
