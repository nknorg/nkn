# NKN: a Scalable Self-Evolving and Self-Incentivized Decentralized Network

> A project aiming to rebuild the Internet that will be truly open,
  decentralized, dynamic, safe, shared and owned by the community.

[![nkn.org](doc/introduction/img/nkn_logo.png)](https://nkn.org/)

[https://nkn.org/](https://nkn.org/)

New to NKN? Watch a [2-minutes video of NKN
concept](https://youtu.be/cT29i3-ImQk) to get started.

[![NKN Introduction](doc/introduction/img/nkn_intro_video.png)](https://youtu.be/cT29i3-ImQk)

NKN is a new generation of highly scalable, self-evolving and
self-incentivized blockchain network infrastructure. NKN tokenizes
network connectivity and data transmission capacity by a novel and
useful Proof of Work. See our [(non-technical) introduction to
NKN](doc/introduction) and our
[Whitepaper](https://www.nkn.org/doc/NKN_Whitepaper.pdf) for more
information.

The core of the NKN network are many connected nodes. Every node is
only connected to and aware of a few other nodes called neighbors. The
topology of the network is verifiable. Data can be transmitted from
any node to any other node in an efficient and verifiable route. The
network is dynamic such that any node can join and leave the network
at any time without causing system failures. Data can also be sent
from and to client (which only sends and receives data but does not
relay data or participate in consensus) without public IP address.

The relay workload can be verified using our Proof of Relay (PoR)
algorithm. A small and fixed portion of the packets will be randomly
selected as proof. The random selection can be verified and cannot be
predicted or controlled. Proof will be sent to other nodes for payment
and rewards.

Mining in NKN network is relaying data to generate proof of relay. The
expected amount of mining rewards is proportional to the amount of
data relayed. Proof of Relay is a useful proof of work without wasting
computational power.

For more technical design details please see our [Technical Design
Document](doc/tech_design_doc).

See also:
* [Wiki](https://github.com/nknorg/nkn/wiki)

## Building

The requirements to build are:
* Go version 1.8 or later
* Glide (a third-party package management tool)
* Properly configured Go environment

Create directory $GOPATH/src/github.com/nknorg/ if not exists

In directory $GOPATH/src/github.com/nknorg/ clone the repository

```shell
$ git clone https://github.com/nknorg/nkn.git
```

Fetch the dependent third-party packages with glide

````shell
$ cd nkn
$ glide install
````

Build the source code with make

```shell
$ make
```

After building the source code, you should see two executable
programs:

* `node`: the node program
* `nodectl`: command line tool for node control

## Deployment

Create four directories to save exectuable files `node` `nodectl` and
config file `config.json`.

``` shell
$ tree
.
├── n1
│   ├── config.json
│   ├── node
│   └── nodectl
├── n2
│   ├── config.json
│   ├── node
│   └── nodectl
├── n3
│   ├── config.json
│   ├── node
│   └── nodectl
├── n4
│   ├── config.json
│   ├── node
│   └── nodectl
```

Create new wallet in each directory

``` shell
$ ./nodectl wallet -c
Password:
Re-enter Password:
Address                            Public Key
-------                            ----------
AbgUvnaiDYbwmKEwSH532W3LPB8Ma2aYYx 0306dd2db26e3cfde2dbe5c8a17ea7c27f13f99c19e2cb59bc13e2d0c41589c7f1
```

Config four public key strings to each `config.json` file, for
example:

```shell
[node 1]

{
  "Configuration": {
    "Magic": 99281,
    "Version": 1,
    "HttpInfoPort": 10333,
    "HttpInfoStart": true,
    "HttpRestPort": 10334,
    "HttpWsPort": 10335,
    "HttpJsonPort": 10336,
    "NodePort": 10338,
    "PrintLevel": 5,
    "IsTLS": false,
    "MultiCoreNum": 4,
    "SeedList": [
      "127.0.0.1:10338",
      "127.0.0.1:20338",
      "127.0.0.1:30338",
      "127.0.0.1:40338"
    ],
    "BookKeepers": [
      "0306dd2db26e3cfde2dbe5c8a17ea7c27f13f99c19e2cb59bc13e2d0c41589c7f1",
      "0259cddbcf52c7d33c84806ddf0c93dc7af3e378cdfaf2c9ef1277a2bf8e483894",
      "0368cfe77148cf122b9a8928c28f99930d578ff7966ab4c4a61be1a8accc8478f5",
      "03712d8dd4714f7459a1db3eef4950bcd359da21a0bda41bd6f721afce7f653051"
    ]
  }
}
```

```shell
[node 2]

{
  "Configuration": {
    "Magic": 99281,
    "Version": 1,
    "HttpInfoPort": 20333,
    "HttpInfoStart": true,
    "HttpRestPort": 20334,
    "HttpWsPort": 20335,
    "HttpJsonPort": 20336,
    "NodePort": 20338,
    "PrintLevel": 5,
    "IsTLS": false,
    "MultiCoreNum": 4,
    "SeedList": [
      "127.0.0.1:10338",
      "127.0.0.1:20338",
      "127.0.0.1:30338",
      "127.0.0.1:40338"
    ],
    "BookKeepers": [
      "0306dd2db26e3cfde2dbe5c8a17ea7c27f13f99c19e2cb59bc13e2d0c41589c7f1",
      "0259cddbcf52c7d33c84806ddf0c93dc7af3e378cdfaf2c9ef1277a2bf8e483894",
      "0368cfe77148cf122b9a8928c28f99930d578ff7966ab4c4a61be1a8accc8478f5",
      "03712d8dd4714f7459a1db3eef4950bcd359da21a0bda41bd6f721afce7f653051"
    ]
  }
}
```

Note that different `config.json` have different ports to avoid port
conflict on one host.

Start all nodes

```shell
$ ./node
Password:
```

## Contributing

Can I contribute patches to NKN project?

Yes! Please open a pull request with signed-off commits. We appreciate
your help!

Please follow our [Golang Style
Guide](doc/specifications/nkn_coding_guide) for coding style.

Please sign off your commit. If you don't sign off your patches, we
will not accept them. This means adding a line that says
"Signed-off-by: Name <email>" at the end of each commit, indicating
that you wrote the code and have the right to pass it on as an open
source patch. This can be done automatically by adding -s when
committing:

```shell
git commit -s
```

Also, please write good git commit messages. A good commit message
looks like this:

	Header line: explain the commit in one line (use the imperative)

	Body of commit message is a few lines of text, explaining things
	in more detail, possibly giving some background about the issue
	being fixed, etc etc.

	The body of the commit message can be several paragraphs, and
	please do proper word-wrap and keep columns shorter than about
	74 characters or so. That way "git log" will show things
	nicely even when it's indented.

	Make sure you explain your solution and why you're doing what you're
	doing, as opposed to describing what you're doing. Reviewers and your
	future self can read the patch, but might not understand why a
	particular solution was implemented.

	Reported-by: whoever-reported-it
	Signed-off-by: Your Name <youremail@yourhost.com>

## License

NKN is released under the terms of the Apache-2.0 license. See
[LICENSE](LICENSE) for more information.
