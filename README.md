[![CircleCI](https://circleci.com/gh/ground-x/klaytn/tree/master.svg?style=svg&circle-token=28de86a436dbe6af811bff7079606433baa43344)](https://circleci.com/gh/ground-x/klaytn/tree/master)
[![codecov](https://codecov.io/gh/ground-x/klaytn/branch/master/graph/badge.svg?token=Tb7cRhQUsU)](https://codecov.io/gh/ground-x/klaytn)

# Klaytn

Official golang implementation of the Klaytn protocol. Please visit [KlaytnDocs](https://docs.klaytn.com/) for more details.

## Building the source

Building the Klaytn node binaries as well as help tools, such as `kcn`, `kpn`, `ken`, `kbn`, `kscn`, `kgen`, `homi`, or `abigen` requires
both a Go (version 1.11.2 or later) and a C compiler.  You can install them using
your favorite package manager.
Once the dependencies are installed, run

    make all   (or make {kcn, kpn, ken, kbn, kscn, kgen, homi, abigen})

## Executables

The klaytn project comes with several wrappers/executables found in the `cmd` directory.

| Command    | Description |
|:----------:|-------------|
| `kcn` | The CLI client for Klaytn Consensus Node. Run `kcn --help` for command-line flags. |
| `kpn` | The CLI client for Klaytn Proxy Node. Run `kpn --help` for command-line flags. |
| `ken` | The CLI client for Klaytn Endpoint Node, which is the entry point into the Klaytn network (main-, test- or private net).  It can be used by other processes as a gateway into the Klaytn network via JSON RPC endpoints exposed on top of HTTP, WebSocket, gRPC, and/or IPC transports. Run `ken --help` for command-line flags. |
| `kscn` | The CLI client for Klaytn ServiceChain Node.  Run `kscn --help` for command-line flags. |
| `kbn` | The CLI client for Klaytn Bootnode. Run `kbn --help` for command-line flags. |
| `kgen` | The CLI client for Klaytn Nodekey Generation Tool. Run `kgen --help` for command-line flags. |
| `homi` | The CLI client for Klaytn Helper Tool to generate initialization files. Run `homi --help` for command-line flags. |
| `abigen` | Source code generator to convert Klaytn contract definitions into easy to use, compile-time type-safe Go packages. |

Both `kcn` and `ken` are capable of running as a full node (default) archive
node (retaining all historical state) or a light node (retrieving data live).

## Running a Core Cell

Core Cell (CC) is a set of consensus node (CN) and one or more proxy nodes
(PNs) and plays a role of generating blocks in the Klaytn network. We recommend to visit
the [CC Operation Guide](https://docs.klaytn.com/node/cc)
for the detail of CC bootstrapping process.

## Running an Endpoint Node

Endpoint Node (EN) is an entry point from the outside of the network in order to
interact with the klaytn network. Currently, two official networks are available: **Baobab** (testnet) and **Cypress** (mainnet). Please visit the official
[EN Operation Guide](https://docs.klaytn.com/node/en).

## Running a Service Chain Node

Service chain node is a node for Service Chain which is an auxiliary blockchain independent from the main chain tailored for individual service requiring special node configurations, customized security levels, and scalability for high throughput services. Service Chain expands and augments Klaytn by providing a data integrity mechanism and supporting token transfers between different chains.
Although the service chain feature is under development, we provide the operation guide for testing purpose. Please visit the official document [Service Chain Operation Guide](https://docs.klaytn.com/node/sc).
Furthermore, for those who are interested in the Klaytn Service Chain, please check out [Klaytn - Service Chain](https://docs.klaytn.com/klaytn/servicechain).

## License

The klaytn library (i.e. all code outside of the `cmd` directory) is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html), also
included in our repository in the `COPYING.LESSER` file.

The klaytn binaries (i.e. all code inside of the `cmd` directory) is licensed under the
[GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html), also included
in our repository in the `COPYING` file.
