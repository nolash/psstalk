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
	psschat "github.com/ethereum/go-ethereum/swarm/pss/protocols/chat"
	"github.com/ethereum/go-ethereum/node"
	pss "github.com/ethereum/go-ethereum/swarm/pss/client"
	"os"

	termbox "github.com/nsf/termbox-go"
)

// command line arguments
var (
	pssclienthost string
	pssclientport int
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
	inC := make(chan *psschat.ChatMsg) // incoming message
	outC := make(chan interface{}) // outgoing message
	connC := make(chan psschat.ChatConn) // connection alerts

	// initialize the terminal overlay handler
	client = talk.NewTalkClient(2)

	// prompt buffers user input
	prompt.Reset()

	// use context for simple teardown
	ctx, cancel = context.WithCancel(context.Background())

	// connect to the pss backend
	// pssclient is a protocol mounted websocket RPC wrapper
	chatlog.Info("Connecting to pss websocket on %s:%d", pssclienthost, pssclientport)
	psc, err = connect(ctx, cancel, inC, outC, connC, pssclienthost, pssclientport)
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
				case chatmsg := <-inC:
					var rs []rune
					var buf *bytes.Buffer
					buf = bytes.NewBuffer(chatmsg.Content)
					for {
						r, n, err := buf.ReadRune()
						if err != nil || n == 0 {
							break
						}
						rs = append(rs, r)
					}
					client.Buffers[1].Add(getSrc(chatmsg.Source), rs)
					otherC <- rs
				case cerr := <-connC:
					crune := []rune(fmt.Sprintf("%s: %s", cerr.Error(), cerr.Detail))
					prompt.Line += (len(crune) / client.Width) + 1
					client.Buffers[0].Add(colorSrc["error"], crune)
					meC <- crune
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
							psc.AddPssPeer(potaddr, psschat.ChatProtocol)
							color = colorSrc["success"]
						} else if client.IsSendCmd() && len(client.Sources) == 0 {
							res = "noone to send to ... add someone first"
							err = fmt.Errorf("no receivers")
						} else {
							if payload != "" {
								// dispatch message
								payload := psschat.ChatMsg{
									Serial:  uint64(client.Buffers[0].Count()),
									Content: []byte(payload),
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

func connect(ctx context.Context, cancel func(), inC chan *psschat.ChatMsg, outC chan interface{}, connC chan psschat.ChatConn, host string, port int) (*pss.Client, error) {
	var err error

	cfg := pss.NewClientConfig()
	cfg.RemoteHost = host
	cfg.RemotePort = port
	pssbackend := pss.NewClient(ctx, cancel, cfg)
	err = pssbackend.Start()
	if err != nil {
		return nil, newError(ePss, err.Error())
	}
	err = pssbackend.RunProtocol(psschat.New(inC, connC, newChatInject(outC)))
	if err != nil {
		return nil, newError(ePss, err.Error())
	}
	return pssbackend, nil
}

func newChatInject(outC chan interface{}) func (*psschat.ChatCtrl) {
	return func(ctrl *psschat.ChatCtrl) {
		if outC != nil {
			go func() {
				for {
					select {
					case msg := <-outC:
						err := ctrl.Peer.Send(msg)
						if err != nil {
							id := ctrl.Peer.ID()
							ctrl.ConnC <- psschat.ChatConn{
								Addr: id[:],
								E: psschat.ESendFail,
								Detail: err,
							}
						}
					}
				}
			}()
		}
	}
}

