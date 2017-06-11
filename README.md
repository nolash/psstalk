# psstalk

pss is short for **Postal Service over Swarm**. It is a [devp2p messaging protocol](https://ethereum.gitbooks.io/frontier-guide/content/devp2p.html) abstraction that allows ethereum nodes that are *not directly connected* to send messages to each other routed by [swarm nodes](https://swarm-guide.readthedocs.io/en/latest/)

**NOTE!** this recipe presumes a basic understanding of running a **go-ethereum** and **swarm** environment. At a minimum, you should have read through everything on this link:

[https://github.com/ethereum/go-ethereum/wiki](https://github.com/ethereum/go-ethereum/wiki)


## ABOUT

**psstalk** is a reference implementation for development using the client package for pss - **Postal Service over Swarm**. For more information on pss and pss client, see here:

[https://github.com/ethersphere/go-ethereum/tree/network-testing-framework-psstalk](https://github.com/ethersphere/go-ethereum/tree/network-testing-framework-psstalk)

It shows how to build a simple chat application, implementing one-to-one and one-to-many messaging.

Aesthetically, psstalk is a tribute to the classic split-screen terminal pipe chat `talk`  for BSD.


## USAGE

Make sure the swarm pss backend is installed and running. All actions equivalent to those listed under **INSTALLATION** below must be completed.

psstalk recognizes the following flags:

    -h <ip> - ip of swarm-pss node
    -p <port> - port of swarm-pss node
    -n <nick> - your nick handle
    -d - show ack and ping in remote viewport
    -i - activate ping send

When successfully started, psstalk shows a terminal screen split in two viewport areas, divided by a horizontal line. The upper part is input area, the lower part is the remote viewport where events and incoming messages are displayed.

### COMMANDS

All commands are preceded by "/". Available commands are:

    /add <handle> <swarm overlay address>

   adds a new peer. you need at least one to send messages

    /msg <handle> <message>

   sends <message> to peer specified by <handle>

    /del <handle>

   removes a peer

Input without a preceding / will be interpreted as a chat message to be sent to all connected peers.

## INSTALLATION

1. **Download go-ethereum**; psstalk uses the go-ethereum swarm binary from the ethersphere repository, branch network-testing-framework-psstalk, located here:

   [https://github.com/ethersphere/go-ethereum/tree/network-testing-framework-psstalk](https://github.com/ethersphere/go-ethereum/tree/network-testing-framework-psstalk)

2. **compile the swarm binary**

   `go build -o <swarm-bin> -tags psschat -v <repo-root>/cmd/swarm`

### BACKEND CONFIGURATION

The minimal command to start a swarm node is:

   `swarm --pss --ethapi ''`

To do anything useful, you need two nodes that connect to each other. If you need to run both nodes on the same host, you need to specify additional params:

     --port <port> - the swarm API port
     --pssport <port> - pss websocket listen port

If you are running the nodes on different hosts, you will want to specify a different host than the default *localhost* so the nodes can find each other:

     --psshost <hostname or ip> - pss websocket listen address
     --nat extip:<ip> - external devp2p address (enables ip in the enode string)

Here is a [shell script](http://swarm-gateways.net/bzzr:/847e6cfbf47949bbed3ecafadbf447b8a0b0a7b6e7ffc09d9cb01e2184243a69/?content_type=text/plain) that (hopefully) simplifies starting a swarm node. 

### CONNECTING THE NODES

Once you've started the nodes you will need two things from their logs:

* the enode string (`grep -i enode:// <log>`)
* the swarm overlay address (`grep -i "bzz address" <log>`)

Connect the nodes by issuing `admin.addPeer("<enode string")` using `geth`, interactively or as follows:

    `geth --exec 'admin.addPeer("<enode string>") attach <path-to-bzzd.ipc>`

You can verify that they are connected by inspecting the peer list:

    `geth --exec 'admin.peers' attach <path-to-bzzd.ipc>`

## DEPENDENCIES

- [go-ethereum](https://github.com/ethersphere/go-ethereum/tree/psstalk) (branch ethersphere/network-testing-framework-psstalk)
- [termbox-go](https://github.com/nsf/termbox-go)

## ISSUES

Even if a handle is specificed upon adding a peer, it will be overwriten by the remote peer's nick.
