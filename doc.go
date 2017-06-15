// psstalk is a reference application for a practical use of the pss client abstraction.
// It is a chat application, where psstalk clients connected to swarm nodes running pss can send and receive messages to and from other psstalk clients connected in the same way.
//
// USAGE
//
// For backend you need a running swarm node from the https://github.com/ethersphere/network-testing-framework-psstalk branch, compiled with the flag "-tags psschat"
//
// Then download and compile this client, and run it with following parameters
//
//    -h - host to connect to (default: 127.0.0.1)
//    -p - port to connect to (default: 8546)
//    -n - your nickname
//    -i - enable ping (poc, not needed for functionality)
//    -d - debug (show ping and ack in incoming viewport)
//
// COMMANDS
//
// The interface is a console screen split in two. In the top half you write commands or messages. In the bottom half you see your incoming messages.
//
// If you write something without a preceding command in the top viewport, the content is treated as a message, and sent to all pss peers that you are "connected" to.
//
// Following commands are available:
//
//    /self - shows your own swarm overlay address
//    /add <handle> <overlayaddress> - adds a "connection" to pss peer from now on called <handle>
//    /del <handle> - remove "connection" to a pss peer called <handle>
//    /list - show list of "connected" peers
//    /msg <handle> <content> - direct-message <content> to pss peer called <handle>
//
// To QUIT the application, press ESC
//
// CONNECTIONS
//
// Note that in pss the notion of "connection" is not a confirmed connection between two peers. There is nothing that will tell you whether the peer is online, or even actually exists. Thus, connections are merely opinions on the party claiming there is one.
//
// the psstalk code aims to demonstrate how a ping/pong and acknowledgements of messages can maintain a notion of connection on a protocol level. 
package main
