package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"context"
	"flag"
	"github.com/nolash/psstalk/talk"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/pot"
	"github.com/nolash/psstalk/protocols/chat"
	"github.com/ethereum/go-ethereum/node"
	pss "github.com/ethereum/go-ethereum/swarm/pss/client"
	"os"
	"time"

	termbox "github.com/nsf/termbox-go"
)

// command line arguments
var (
	pssclienthost string
	pssclientport int
	pssdebug bool
	pssnick string
	pssping bool
)

var (
	debugcount int
	myNick string = "self"
	run bool = true
	freeze bool = false
	chatlog log.Logger
)

func init() {
	hs := log.StreamHandler(os.Stderr, log.TerminalFormat(true))
	hf := log.LvlFilterHandler(log.LvlTrace, hs)
	h := log.CallerFileHandler(hf)
	log.Root().SetHandler(h)
	chatlog = log.New("chatlog", "main")

	flag.StringVar(&pssclienthost, "h", "localhost", "pss websocket hostname")
	flag.IntVar(&pssclientport, "p", node.DefaultWSPort, "pss websocket port")
	flag.BoolVar(&pssdebug, "d", false, "output ping and ack")
	flag.StringVar(&pssnick, "n", "", "public nick")
	flag.BoolVar(&pssping, "i", false, "activate ping")
	flag.Parse()
}

func main() {
	var err error
	var ctx context.Context
	var cancel func()
	var psc *pss.Client

	quitC := make(chan struct{})

	// screen update trigger channels
	meC := make(chan []rune) // I send a message
	otherC := make(chan []rune) // Others send me a message
	promptC := make(chan bool) // Keyboard input

	// peer channels
	inC := make(chan interface{}) // incoming message
	outC := make(chan interface{}) // outgoing message
	connC := make(chan *chat.ChatConn) // connection alerts
	tmpC := make(chan *incomingMsg) // temporary message channel to update ping map to discoveryid

	// initialize the terminal overlay handler
	client = talk.NewTalkClient(2)

	// prompt buffers user input
	prompt.Reset()

	// use context for simple teardown
	ctx, cancel = context.WithCancel(context.Background())


	// connect to the pss backend
	// pssclient is a protocol mounted websocket RPC wrapper
	chatlog.Info("Connecting to pss websocket", "host", pssclienthost, "port", pssclientport)
	psc, err = connect(ctx, cancel, pssnick, inC, tmpC, outC, connC, pssclienthost, pssclientport)
	if err != nil {
		chatlog.Crit(err.Error())
		os.Exit(1)
	}

	// start the termbox display
	err = startup()
	if err != nil {
		chatlog.Crit(err.Error())
		os.Exit(1)
	}

	// handle incoming messages
	go func() {
		for run {
			select {
				case incoming := <-tmpC:
					chatsrc := unknownSrc
					var rs []rune
					chatmsg, ok := incoming.msg.(*chat.ChatMsg)
					if ok {
						addr, err := talk.StringToAddress(fmt.Sprintf("%x", incoming.addr))
						if err == nil {
							lookupsrc := client.GetSourceByAddress(addr)
							if lookupsrc != nil {
								chatsrc = lookupsrc
							} else {
								chatlog.Warn("failed lookup src", "addr", addr, "original addr", incoming.addr)
							}
						}
						rs = bytes.Runes([]byte(chatmsg.Content))
					} else {
						chatack, ok := incoming.msg.(*chat.ChatAck)
						if ok && pssdebug {
							rs = []rune(fmt.Sprintf("got ack for msg %d", chatack.Serial))
							chatsrc = colorSrc["notify"]
						} else {
							break
						}
					}
					client.Buffers[1].Add(chatsrc, rs)
					otherC <- rs
				case cerr := <-connC:
					var rs []rune

					colorsrc := colorSrc["error"]
					if cerr.E == chat.EPing {
						if !pssdebug {
							break
						}
						rs = []rune(fmt.Sprintf("got ping from %x", cerr.Addr[:]))
					} else if cerr.E == chat.EHandshake {
						addr, err := talk.StringToAddress(fmt.Sprintf("%x", cerr.Addr))
						if err != nil {
							chatlog.Error("invalid address in handshake", "addr", cerr.Addr)
						}
						chatlog.Debug("checking source for addr", "addr", addr, "orig", cerr.Addr)

						src := client.GetSourceByAddress(addr)
						if src == nil {
							_, _, err := client.Process([]rune(fmt.Sprintf("/add %s %x", cerr.Detail, cerr.Addr)))
							if err != nil {
								chatlog.Error("internal error in adding peer to talk client", "err", err)
							}
							rs = []rune(fmt.Sprintf("new contact %s connected. Yay :)", cerr.Detail))
						} else if src.Nick != cerr.Detail {
							rs = []rune(fmt.Sprintf("%s connected and is now known as %s!", src.Nick, cerr.Detail))
							src.Nick = cerr.Detail
						} else {
							rs = []rune(fmt.Sprintf("%s reconnected!", cerr.Detail))
						}

						colorsrc = colorSrc["success"]

					} else {
						rs = []rune(fmt.Sprintf("%s: %s", cerr.Error(), cerr.Detail))
					}
					client.Buffers[1].Add(colorsrc, rs)
					otherC <- rs
			}
		}
	}()

	// update terminal screen loop
	go func() {

		for run {
			select {
			case <-meC:
				updateView(client.Buffers[0], 0, client.Lines[0]-1)
				termbox.Flush()
			case <-promptC:
				termbox.SetCursor(prompt.Count%client.Width, prompt.Line+(prompt.Count/client.Width))
				termbox.Flush()
			case <-otherC:
				updateView(client.Buffers[1], client.Lines[0]+1, client.Lines[1])
				termbox.Flush()
			case <-quitC:
				run = false
			}
		}
	}()

	// handle input

	termbox.SetCursor(0, 0)

	for run {
		before := prompt.Count / client.Width
		ev := termbox.PollEvent()
		if ev.Type == termbox.EventKey {
			if ev.Ch == 0 {
				switch ev.Key {
				// esc quits the application
				case termbox.KeyEsc:
					quitC <- struct{}{}
					run = false
				// pop from prompt buffer
				// if the line count changes also update the message buffer, less the lines that the prompt buffer occupies
				case termbox.KeyBackspace:
					removeFromPrompt(before)
					promptC <- true
				case termbox.KeyBackspace2:
					removeFromPrompt(before)
					promptC <- true
				// enter sends the message
				case termbox.KeyEnter:
					if len(prompt.Buffer) == 0 {
						break
					}
					line := prompt.Buffer
					res, payload, err := client.Process(line)
					color := colorSrc["error"]
					if err == nil {
						if client.IsAddCmd() {
							args := client.GetCmd()
							b, _ := hex.DecodeString(args[1])
							potaddr := pot.Address{}
							copy(potaddr[:], b[:])
							psc.AddPssPeer(potaddr, chat.ChatProtocol)
							color = colorSrc["success"]
						} else if client.IsSendCmd() && len(client.Sources) == 0 {
							res = "noone to send to ... add someone first"
							err = fmt.Errorf("no receivers")
						} else {
							if payload != "" {
								// dispatch message
								serial++
								payload := chat.ChatMsg{
									Serial:  uint64(serial),
									//Content: []byte(payload),
									Content: payload,
									Source:  randomSrc().Nick,
								}

								outC <- payload

								// add the line to the history buffer for the local user
								client.Buffers[0].Add(nil, line)
								color = colorSrc["success"]
							}

						}

					}
					if len(res) > 0 {
						resrunes := bytes.Runes([]byte(res))
						client.Buffers[0].Add(color, resrunes)
						otherC <- resrunes
					}
					// move the prompt line down
					// and back up if we hit the bottom of the viewport height
					prompt.Line += (prompt.Count / client.Width) + 1
					if prompt.Line > client.Lines[0]-1 {
						prompt.Line = client.Lines[0] - 1
					}

					// update the local user viewport
					meC <- line

					// clear the prompt buffer
					prompt.Reset()

					// clear the prompt line in the viewport
					for i := 0; i < client.Width; i++ {
						termbox.SetCell(i, prompt.Line, runeSpace, bgAttr, bgAttr)
					}

					// update the prompt in the viewport
					// (do we need this?)
					promptC <- true

				case termbox.KeySpace:
					addToPrompt(runeSpace, before)
					promptC <- true
				}
			} else {
				addToPrompt(ev.Ch, before)
				promptC <- true

			}
		}
	}

	shutdown()
}

func connect(ctx context.Context, cancel func(), nick string, inC chan interface{}, tmpC chan *incomingMsg, outC chan interface{}, connC chan *chat.ChatConn, host string, port int) (*pss.Client, error) {
	var err error

	cfg := pss.NewClientConfig()
	cfg.RemoteHost = host
	cfg.RemotePort = port
	pssbackend := pss.NewClient(ctx, cancel, cfg)
	err = pssbackend.Start()
	if err != nil {
		return nil, newError(ePss, err.Error())
	}
	oaddr := pssbackend.BaseAddr
	err = pssbackend.RunProtocol(chat.New(oaddr, nick, inC, connC, newChatInject(tmpC, outC)))
	if err != nil {
		return nil, newError(ePss, err.Error())
	}
	return pssbackend, nil
}

func newChatInject(tmpC chan *incomingMsg, outC chan interface{}) func (*chat.ChatCtrl) {
	return func(ctrl *chat.ChatCtrl) {
		if outC != nil {
			go func() {
				for {
					select {
					case msg := <-outC:
						err := ctrl.Peer.Send(msg)
						if err != nil {
							id := ctrl.Peer.ID()
							ctrl.ConnC <- &chat.ChatConn{
								Addr: id[:],
								E: chat.ESendFail,
								Detail: err.Error(),
							}
						}
					}
				}
			}()
		}
		go func() {
			checknext := time.NewTimer(pinginterval)
			for {
				select {
					case <-checknext.C:
						if pssping {
							chatlog.Debug("sending ping", "peer", ctrl.Peer)
							ctrl.Peer.Send(&chat.ChatPing{})
						}
					case msg := <-ctrl.InC:
						chatlog.Debug("incoming msg", "peer", ctrl.Peer, "msg", msg)
						tmpC <- &incomingMsg{
							msg: msg,
							addr: ctrl.PeerOAddr,

						}
						tmppingtracker[ctrl.Peer] = time.Now()
				}
				checknext.Reset(pinginterval)
			}
		}()
	}
}

