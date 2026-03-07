# The devp2p command

The devp2p command line tool is a utility for low-level peer-to-peer debugging and
protocol development purposes. It can do many things.

### QNR Decoding

Use `devp2p qnrdump <base64>` to verify and display a QRL Node Record.

### Node Key Management

The `devp2p key ...` command family deals with node key files.

Run `devp2p key generate mynode.key` to create a new node key in the `mynode.key` file.

Run `devp2p key to-qnode mynode.key -ip 127.0.0.1 -tcp 30303` to create a qnode:// URL
corresponding to the given node key and address information.

### Maintaining DNS Discovery Node Lists

The devp2p command can create and publish DNS discovery node lists.

Run `devp2p dns sign <directory>` to update the signature of a DNS discovery tree.

Run `devp2p dns sync <qnrtree-URL>` to download a complete DNS discovery tree.

Run `devp2p dns to-cloudflare <directory>` to publish a tree to CloudFlare DNS.

Run `devp2p dns to-route53 <directory>` to publish a tree to Amazon Route53.

You can find more information about these commands in the [DNS Discovery Setup Guide][dns-tutorial].

### Node Set Utilities

There are several commands for working with JSON node set files. These files are generated
by the discovery crawlers and DNS client commands. Node sets also used as the input of the
DNS deployer commands.

Run `devp2p nodeset info <nodes.json>` to display statistics of a node set.

Run `devp2p nodeset filter <nodes.json> <filter flags...>` to write a new, filtered node
set to standard output. The following filters are supported:

- `-limit <N>` limits the output set to N entries, taking the top N nodes by score
- `-ip <CIDR>` filters nodes by IP subnet
- `-min-age <duration>` filters nodes by 'first seen' time
- `-qrl-network <mainnet>` filters nodes by "qrl" QNR entry
- `-snap` filters nodes by snap protocol support

For example, given a node set in `nodes.json`, you could create a filtered set containing
up to 20 qrl mainnet nodes which also support snap sync using this command:

    devp2p nodeset filter nodes.json -eth-network mainnet -snap -limit 20

### Discovery v4 Utilities

The `devp2p discv4 ...` command family deals with the [Node Discovery v4][discv4]
protocol.

Run `devp2p discv4 ping <qnode/QNR>` to ping a node.

Run `devp2p discv4 resolve <qnode/QNR>` to find the most recent node record of a node in
the DHT.

Run `devp2p discv4 crawl <nodes.json path>` to create or update a JSON node set.

### Discovery v5 Utilities

The `devp2p discv5 ...` command family deals with the [Node Discovery v5][discv5]
protocol. This protocol is currently under active development.

Run `devp2p discv5 ping <QNR>` to ping a node.

Run `devp2p discv5 resolve <QNR>` to find the most recent node record of a node in
the discv5 DHT.

Run `devp2p discv5 listen` to run a Discovery v5 node.

Run `devp2p discv5 crawl <nodes.json path>` to create or update a JSON node set containing
discv5 nodes.

### Discovery Test Suites

The devp2p command also contains interactive test suites for Discovery v4 and Discovery
v5.

To run these tests against your implementation, you need to set up a networking
environment where two separate UDP listening addresses are available on the same machine.
The two listening addresses must also be routed such that they are able to reach the node
you want to test.

For example, if you want to run the test on your local host, and the node under test is
also on the local host, you need to assign two IP addresses (or a larger range) to your
loopback interface. On macOS, this can be done by executing the following command:

    sudo ifconfig lo0 add 127.0.0.2

You can now run either test suite as follows: Start the node under test first, ensuring
that it won't talk to the Internet (i.e. disable bootstrapping). An easy way to prevent
unintended connections to the global DHT is listening on `127.0.0.1`.

Now get the QNR of your node and store it in the `NODE` environment variable.

Start the test by running `devp2p discv5 test -listen1 127.0.0.1 -listen2 127.0.0.2 $NODE`.

### QRL Protocol Test Suite

The QRL Protocol test suite is a conformance test suite for the qrl protocol.

To run the qrl protocol test suite against your implementation, the node needs to be initialized as such:

1. initialize the gqrl node with the `genesis.json` file contained in the `testdata` directory
2. import the `halfchain.rlp` file in the `testdata` directory
3. run gqrl with the following flags:
```
gqrl --datadir <datadir> --nodiscover --nat=none --networkid 19763 --verbosity 5
```

Then, run the following command, replacing `<qnode>` with the qnode of the gqrl node:
 ```
 devp2p rlpx qrl-test <qnode> cmd/devp2p/internal/qrltest/testdata/chain.rlp cmd/devp2p/internal/qrltest/testdata/genesis.json
```

Repeat the above process (re-initialising the node) in order to run the QRL Protocol test suite again.

[eth]: https://github.com/ethereum/devp2p/blob/master/caps/eth.md
[dns-tutorial]: https://geth.ethereum.org/docs/developers/geth-developer/dns-discovery-setup
[discv4]: https://github.com/ethereum/devp2p/tree/master/discv4.md
[discv5]: https://github.com/ethereum/devp2p/tree/master/discv5/discv5.md
