package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/nolash/psstalk/term"
	termbox "github.com/nsf/termbox-go"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/pot"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pss"
	pssclient "github.com/ethereum/go-ethereum/swarm/pss/client"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

var (
	services                              = newServices()
	pssServiceName                        = "pss"
	bzzServiceName                        = "bzz"
	unsentColor         termbox.Attribute = termbox.ColorWhite
	pendingColor        termbox.Attribute = termbox.ColorDefault
	successColor        termbox.Attribute = termbox.ColorGreen
	failColor           termbox.Attribute = termbox.ColorRed
	maxRandomLineLength int
	minRandomLineLength int
	chatlog             log.Logger
	fakenodeconfigs     []*adapters.NodeConfig
	fakepots            = make(map[discover.NodeID]pot.Address)
	outCs               = make(map[discover.NodeID]chan interface{})
	inCs                = make(map[discover.NodeID]chan interface{})
)

// initialize the client buffer handler
// draw the mid screen separator
func init() {
	var i int
	var err error

	client = term.NewTalkClient(2)

	err = termbox.Init()
	if err != nil {
		panic("could not init termbox")
	}
	err = termbox.Clear(termbox.ColorYellow, termbox.ColorBlack)
	if err != nil {
		fmt.Printf("cant clear %v", err)
		os.Exit(1)
	}
	updateSize()
	for i := 0; i < client.Width; i++ {
		termbox.SetCell(i, client.Lines[0], runeDash, termbox.ColorYellow, termbox.ColorBlack)
	}
	termbox.Flush()

	hs := log.StreamHandler(os.Stderr, log.TerminalFormat(true))
	hf := log.LvlFilterHandler(log.LvlTrace, hs)
	h := log.CallerFileHandler(hf)
	log.Root().SetHandler(h)

	chatlog = log.New("chatlog", "main")
	srcFormat = make(map[*term.TalkSource]termbox.Attribute)

	for i = 0; i < 3; i++ {
		var potaddr pot.Address
		fakenodeconfig := adapters.RandomNodeConfig()
		fakenodeconfig.Services = []string{"bzz", "pss"}
		//fakenodeconfig.Services = []string{"pss"}
		fakenodeconfigs = append(fakenodeconfigs, fakenodeconfig)
		addr := network.ToOverlayAddr(fakenodeconfig.ID[:])
		copy(potaddr[:], addr[:])
		fakepots[fakenodeconfig.ID] = potaddr
		inCs[fakenodeconfig.ID] = make(chan interface{})
		outCs[fakenodeconfig.ID] = make(chan interface{})
	}

	adapters.RegisterServices(services)
}

func TestFoo(t *testing.T) {
	t.Fatalf("Only run one of the tests, please")
}

func TestRandomOutput(t *testing.T) {
	//var err error
	logC := make(chan []rune)
	chatC := make(chan []rune)
	quitTickC := make(chan struct{})
	quitC := make(chan struct{})

	rand.Seed(time.Now().Unix())

	addSrc("one", "bob", termbox.ColorYellow)
	addSrc("other", "alice", termbox.ColorGreen)

	logticker := time.NewTicker(time.Millisecond * 250)
	chatticker := time.NewTicker(time.Millisecond * 600)

	go func() {
		for i := 0; run; i++ {
			r := randomLine([]rune{int32((i/10)%10) + 48, int32(i%10) + 48, 46, 46, 46, 32}, 0)
			select {
			case <-chatticker.C:
				client.Buffers[0].Add(randomSrc(), r)
				chatC <- r
			case <-logticker.C:
				client.Buffers[1].Add(nil, r)
				logC <- r
			case <-quitTickC:
				logticker.Stop()
				chatticker.Stop()
				break
			}
		}
	}()

	go func() {
		for run {
			select {
			case <-chatC:
				updateView(client.Buffers[0], 0, client.Lines[0])
				termbox.Flush()
			case <-logC:
				updateView(client.Buffers[1], client.Lines[0]+1, client.Lines[1])
				termbox.Flush()
			case <-quitC:
				run = false
			}
		}
	}()

	for run {
		ev := termbox.PollEvent()
		if ev.Type == termbox.EventKey {
			if freeze {
				quitC <- struct{}{}
				break
			} else {
				quitTickC <- struct{}{}
				freeze = true
			}
		}
	}

	shutdown(t, nil)
}

func TestInputAndRandomOutput(t *testing.T) {
	//var err error
	meC := make(chan []rune)
	otherC := make(chan []rune)
	promptC := make(chan bool)
	quitTickC := make(chan struct{})
	quitC := make(chan struct{})

	prompt.Reset()

	rand.Seed(time.Now().Unix())

	otherticker := time.NewTicker(time.Millisecond * 2500)

	go func() {
		for i := 0; run; i++ {
			r := randomLine([]rune{int32((i/10)%10) + 48, int32(i%10) + 48, 46, 46, 46, 32}, 0)
			select {
			case <-otherticker.C:
				client.Buffers[1].Add(randomSrc(), r)
				otherC <- r
			case <-quitTickC:
				otherticker.Stop()
				break
			}
		}
	}()

	go func() {
		for run {
			select {
			case <-meC:
				updateView(client.Buffers[0], 0, client.Lines[0]-1)
				termbox.Flush()
			case <-otherC:
				updateView(client.Buffers[1], client.Lines[0]+1, client.Lines[1])
				termbox.Flush()
			case <-promptC:
				termbox.SetCursor(prompt.Count%client.Width, prompt.Line+(prompt.Count/client.Width))
				termbox.Flush()
			case <-quitC:
				run = false
			}
		}
	}()

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
				case termbox.KeyEnter:
					line := prompt.Buffer
					client.Buffers[0].Add(nil, line)
					prompt.Line += (prompt.Count / client.Width) + 1
					if prompt.Line > client.Lines[0]-1 {
						prompt.Line = client.Lines[0] - 1
					}
					meC <- line
					prompt.Reset()
					for i := 0; i < client.Width; i++ {
						termbox.SetCell(i, prompt.Line, runeSpace, bgAttr, bgAttr)
					}
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

	shutdown(t, nil)
}

func TestPssReceive(t *testing.T) {
	var err error
	otherC := make(chan []rune)
	quitTickC := make(chan struct{})
	pssInC := inCs[fakenodeconfigs[0].ID]
	pssOutC := outCs[fakenodeconfigs[0].ID]
	quitC := make(chan struct{})
	quitPssC := make(chan struct{})
	txtreplies := [][]byte{
		[]byte("hello"),
		[]byte("here"),
		[]byte("is"),
		[]byte("pss!"),
	}

	addSrc("only", "bob", termbox.ColorYellow)

	prompt.Reset()

	otherticker := time.NewTicker(time.Millisecond * 500)

	proto := newProtocol(pssInC, pssOutC)

	ctx, cancel := context.WithCancel(context.Background())
	_, ps := newPss(t, ctx, cancel, proto, quitPssC)

	err = pss.RegisterPssProtocol(ps, &chatTopic, chatProtocol, proto)
	if err != nil {
		t.Fatalf("pss create fail: %v", err)
	}

	// set up the message to send
	code, ok := chatProtocol.GetCode(&chatMsg{})
	if !ok {
		t.Fatalf("get chatmsg code fail!")
	}
	go func() {
		serial := 1
		for run {
			select {
			case <-otherticker.C:
				payload := chatMsg{
					Serial:  uint64(serial),
					Content: txtreplies[(serial-1)%len(txtreplies)],
					Source:  "bar",
				}
				pmsg, err := pss.NewProtocolMsg(code, payload)
				if err != nil {
					t.Fatalf("new protocomsg fail: %v", err)
				}

				env := pss.NewPssEnvelope(fakepots[fakenodeconfigs[1].ID].Bytes(), chatTopic, pmsg)

				ps.Process(&pss.PssMsg{
					To:      fakepots[fakenodeconfigs[0].ID].Bytes(),
					Payload: env,
				})
				serial++
			case msg := <-pssInC:
				chatmsg, ok := msg.(*chatMsg)
				if !ok {
					chatlog.Crit("Could not parse chatmsg", "msg", msg)
					quitC <- struct{}{}
				}
				r := []rune{int32(chatmsg.Serial + 0x30), int32(0x3D)}
				for _, b := range chatmsg.Content {
					r = append(r, int32(b))
				}
				client.Buffers[1].Add(randomSrc(), r)
				otherC <- r
				if chatmsg.Serial > 9 {
					quitC <- struct{}{}
				}
			case <-quitTickC:
				run = false
				otherticker.Stop()
				quitPssC <- struct{}{}
				break
			}
		}
	}()

	go func() {
		for run {
			select {
			case <-otherC:
				updateView(client.Buffers[1], client.Lines[0]+1, client.Lines[1])
				termbox.Flush()
			}
		}
	}()
	<-quitC
	quitTickC <- struct{}{}
	shutdown(t, nil)
}

func TestPssSendAndReceive(t *testing.T) {
	var potaddr pot.Address
	var serial int = 1
	meC := make(chan []rune)
	otherC := make(chan []rune)
	promptC := make(chan bool)
	pssInC := inCs[fakenodeconfigs[0].ID]
	pssOutC := outCs[fakenodeconfigs[0].ID]
	quitC := make(chan struct{})
	quitPssC := make(chan struct{})

	addSrc(fmt.Sprintf("%x", fakepots[fakenodeconfigs[0].ID].Bytes()[:8]), "bob", termbox.ColorYellow)

	prompt.Reset()

	proto := newProtocol(pssInC, pssOutC)

	ctx, cancel := context.WithCancel(context.Background())
	psc, _ := newPss(t, ctx, cancel, proto, quitPssC)

	copy(potaddr[:], fakepots[fakenodeconfigs[0].ID].Bytes())

	psc.AddPssPeer(potaddr, chatProtocol)

	go func() {
		for run {
			select {
			case msg := <-pssInC:
				var rs []rune
				var buf *bytes.Buffer
				chatmsg, ok := msg.(*chatMsg)
				if !ok {
					chatlog.Crit("Could not parse chatmsg", "msg", msg)
					quitC <- struct{}{}
				}
				buf = bytes.NewBufferString(fmt.Sprintf("%d=", chatmsg.Serial))
				for {
					r, n, err := buf.ReadRune()
					if err != nil || n == 0 {
						break
					}
					rs = append(rs, r)
				}

				buf = bytes.NewBuffer(chatmsg.Content)
				for {
					r, n, err := buf.ReadRune()
					if err != nil || n == 0 {
						break
					}
					rs = append(rs, r)
				}
				client.Buffers[1].Add(randomSrc(), rs)
				otherC <- rs
			}
		}
	}()

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
				case termbox.KeyEnter:
					var b []byte
					buf := bytes.NewBuffer(b)

					line := prompt.Buffer
					client.Buffers[0].Add(nil, line)
					prompt.Line += (prompt.Count / client.Width) + 1
					if prompt.Line > client.Lines[0]-1 {
						prompt.Line = client.Lines[0] - 1
					}
					meC <- line
					prompt.Reset()
					for i := 0; i < client.Width; i++ {
						termbox.SetCell(i, prompt.Line, runeSpace, bgAttr, bgAttr)
					}
					promptC <- true

					// serialize the runes in the line to bytes
					for _, r := range line {
						n, err := buf.WriteRune(r)
						if err != nil || n == 0 {
							break
						}
					}

					// send the message to ourselves using pssclient
					payload := chatMsg{
						Serial:  uint64(serial),
						Content: buf.Bytes(),
						Source:  randomSrc().Nick,
					}

					pssOutC <- payload
					serial++

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
	shutdown(t, nil)
}

func TestCur(t *testing.T) {
//func TestPssSendAndReceiveFullLinear(t *testing.T) {

	var conncount int
	var fakenodes []adapters.Node
	fakeclients := make(map[discover.NodeID]*pssclient.PssClient)

	var serialself int32 = 1
	var serialother int32 = 1

	meC := make(chan []rune)
	otherC := make(chan []rune)
	promptC := make(chan bool)
	quitC := make(chan struct{})
	quitRPCC := make(chan struct{})
	readyC := make(chan discover.NodeID)

	txtreplies := [][]byte{
		[]byte("foo"),
		[]byte("bar"),
		[]byte("42"),
	}

	addSrc(fmt.Sprintf("%x", fakepots[fakenodeconfigs[0].ID].Bytes()[:8]), "bob", termbox.ColorYellow)
	addSrc(fmt.Sprintf("%x", fakepots[fakenodeconfigs[1].ID].Bytes()[:8]), "alice", termbox.ColorCyan)

	prompt.Reset()

	ctx, _ := context.WithCancel(context.Background())

	// initialize the network
	adapter := adapters.NewSimAdapter(services)

	simnet := simulations.NewNetwork(adapter, &simulations.NetworkConfig{
		ID: "psschat",
	})
	defer simnet.Shutdown()

	// we need two nodes to connect

	for i, cfg := range fakenodeconfigs {

		chatlog.Debug("fakenode", "idx", i, "addr", fakepots[cfg.ID], "id", fmt.Sprintf("%x", cfg.ID[:8]))
		fakenode, err := simnet.NewNodeWithConfig(cfg)
		if err != nil {
			shutdown(t, fmt.Errorf("couldn't start node: %v", err))
		}
		fakenodes = append(fakenodes, fakenode)

		if err := simnet.Start(fakenode.ID()); err != nil {
			shutdown(t, fmt.Errorf("error starting node: %v", err))
		}

		fakerpc, err := fakenode.Client()
		if err != nil {
			shutdown(t, fmt.Errorf("error getting node rpc: %v", err))
		}

		fakeclients[cfg.ID] = pssclient.NewPssClientWithRPC(ctx, fakerpc)
		fakeclients[cfg.ID].Start()
		fakeclients[cfg.ID].RunProtocol(newProtocol(inCs[cfg.ID], outCs[cfg.ID]))

		peerevents := make(chan *p2p.PeerEvent)
		peersub, err := fakerpc.Subscribe(ctx, "admin", peerevents, "peerEvents")
		if err != nil {
			shutdown(t, fmt.Errorf("error getting peer events for node %v: %s", cfg.ID, err))
		}

		go func() {
			gotit := false
			for {
				select {
					case event := <-peerevents:
						if event.Type == "add" {
							if !gotit {
								readyC <-cfg.ID
								gotit = true
							}
							chatlog.Debug("add", "node", cfg.ID, "peer", event.Peer)
						}
					case <-quitRPCC:
						peersub.Unsubscribe()
						return
				}
			}
		}()
	}

	simnet.Connect(fakenodeconfigs[0].ID, fakenodeconfigs[2].ID)
	simnet.Connect(fakenodeconfigs[1].ID, fakenodeconfigs[2].ID)

	for conncount < 2 {
		 <-readyC
		conncount++;
	}

	chatlog.Info("connections are up")

	fakeclients[fakenodeconfigs[0].ID].AddPssPeer(fakepots[fakenodeconfigs[1].ID], chatProtocol)
	fakeclients[fakenodeconfigs[1].ID].AddPssPeer(fakepots[fakenodeconfigs[0].ID], chatProtocol)

	// at this point we are connected in a line
	// let's start relaying messages

	go func() {
		for run {
			select {
				case <-inCs[fakenodeconfigs[1].ID]:
					go func() {
						srcpot := fakepots[fakenodeconfigs[1].ID].Bytes()
						dur, err := time.ParseDuration(fmt.Sprintf("%dms", (rand.Int() % 2500) + 500))
						if err != nil {
							chatlog.Error("failed duration parse for reply msg")
							return
						}
						time.Sleep(dur)
						payload := chatMsg{
							Serial:  uint64(serialother),
							Content: txtreplies[rand.Int() % len(txtreplies)],
							Source:  fmt.Sprintf("%x", srcpot[:8]),
						}

						outCs[fakenodeconfigs[1].ID] <- payload
						atomic.AddInt32(&serialother, 1)
					}()

				case msg := <-inCs[fakenodeconfigs[0].ID]:
					var rs []rune
					var buf *bytes.Buffer
					chatmsg, ok := msg.(*chatMsg)
					if !ok {
						chatlog.Crit("Could not parse chatmsg", "msg", msg)
						quitC <- struct{}{}
					}
					buf = bytes.NewBufferString(fmt.Sprintf("%d=", chatmsg.Serial))
					for {
						r, n, err := buf.ReadRune()
						if err != nil || n == 0 {
							break
						}
						rs = append(rs, r)
					}

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
			}
		}
	}()


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
				case termbox.KeyEnter:
					var b []byte
					buf := bytes.NewBuffer(b)

					line := prompt.Buffer
					client.Buffers[0].Add(nil, line)
					prompt.Line += (prompt.Count / client.Width) + 1
					if prompt.Line > client.Lines[0]-1 {
						prompt.Line = client.Lines[0] - 1
					}
					meC <- line
					prompt.Reset()
					for i := 0; i < client.Width; i++ {
						termbox.SetCell(i, prompt.Line, runeSpace, bgAttr, bgAttr)
					}
					promptC <- true

					// serialize the runes in the line to bytes
					for _, r := range line {
						n, err := buf.WriteRune(r)
						if err != nil || n == 0 {
							break
						}
					}

					// send the message to ourselves using pssclient
					payload := chatMsg{
						Serial:  uint64(serialself),
						Content: buf.Bytes(),
						Source:  randomSrc().Nick,
					}

					outCs[fakenodeconfigs[0].ID] <- payload
					serialself++

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

	quitRPCC <- struct{}{}

	shutdown(t, nil)
}

// split the screen vertically in two
func updateSize() {
	var h int
	client.Width, h = termbox.Size()
	client.Lines[0] = h / 2
	client.Lines[1] = client.Lines[0]
	if (client.Lines[0]/2)*2 == client.Lines[0] {
		client.Lines[1]--
	}
	minRandomLineLength = int(float64(client.Width) * 0.4)
	maxRandomLineLength = int(float64(client.Width) * 2.8)
}

// generate a random line of content
func randomLine(prefix []rune, rlinelen int) (rline []rune) {
	b := make([]byte, 1)
	if rlinelen == 0 {
		rlinelen = (rand.Int() % (maxRandomLineLength - minRandomLineLength)) + minRandomLineLength + 1
	}
	for i := 0; i < rlinelen; i++ {
		rand.Read(b)
		b[0] = b[0]%26 + 40
		r, _ := utf8.DecodeRune(b)
		rline = append(rline, r)
	}
	if prefix != nil && len(prefix) <= len(rline) {
		copy(rline[:len(prefix)], prefix[:])
	}
	return
}
func newPss(t *testing.T, ctx context.Context, cancel func(), proto *p2p.Protocol, quitC chan struct{}) (*pssclient.PssClient, *pss.Pss) {
	var err error

	conf := pssclient.NewPssClientConfig()

	psc := pssclient.NewPssClient(ctx, cancel, conf)

	ps := pss.NewTestPss(fakepots[fakenodeconfigs[0].ID].Bytes())

	srv := rpc.NewServer()
	srv.RegisterName("pss", pss.NewPssAPI(ps))
	srv.RegisterName("psstest", pss.NewPssAPITest(ps))
	ws := srv.WebsocketHandler([]string{"*"})
	uri := fmt.Sprintf("%s:%d", "localhost", 8546)

	sock, err := net.Listen("tcp", uri)
	if err != nil {
		t.Fatalf("Tcp (recv) on %s failed: %v", uri, err)
	}

	go func() {
		http.Serve(sock, ws)
	}()

	go func() {
		<-quitC
		sock.Close()
	}()

	psc.Start()
	psc.RunProtocol(proto)

	return psc, ps
}

func newProtocol(inC chan interface{}, outC chan interface{}) *p2p.Protocol {
	chatctrl := chatCtrl{
		inC: inC,
	}
	return &p2p.Protocol{
		Name:    chatProtocol.Name,
		Version: chatProtocol.Version,
		Length:  3,
		Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
			pp := protocols.NewPeer(p, rw, chatProtocol)
			if outC != nil {
				go func() {
					for {
						select {
						case msg := <-outC:
							err := pp.Send(msg)
							if err != nil {
								pp.Drop(err)
								break
							}
						}
					}
				}()
			}
			chatctrl.oAddr = fakepots[p.ID()].Bytes()
			pp.Run(chatctrl.chatHandler)
			return nil
		},
	}
}

func newServices() adapters.Services {
	stateStore := adapters.NewSimStateStore()
	kademlias := make(map[discover.NodeID]*network.Kademlia)
	kademlia := func(id discover.NodeID) *network.Kademlia {
		if k, ok := kademlias[id]; ok {
			return k
		}
		addr := network.NewAddrFromNodeID(id)
		params := network.NewKadParams()
		params.MinProxBinSize = 2
		params.MaxBinSize = 3
		params.MinBinSize = 1
		params.MaxRetries = 1000
		params.RetryExponent = 2
		params.RetryInterval = 1000000
		kademlias[id] = network.NewKademlia(addr.Over(), params)
		return kademlias[id]
	}
	return adapters.Services{
		"pss": func(ctx *adapters.ServiceContext) (node.Service, error) {
			cachedir, err := ioutil.TempDir("", "pss-cache")
			if err != nil {
				return nil, fmt.Errorf("create pss cache tmpdir failed", "error", err)
			}
			dpa, err := storage.NewLocalDPA(cachedir)
			if err != nil {
				return nil, fmt.Errorf("local dpa creation failed", "error", err)
			}

			pssp := pss.NewPssParams(false)
			ps := pss.NewPss(kademlia(ctx.Config.ID), dpa, pssp)

			proto := newProtocol(nil, nil)

			err = pss.RegisterPssProtocol(ps, &chatTopic, chatProtocol, proto)
			if err != nil {
				chatlog.Error("Couldnt register chat protocol in pss service", "err", err)
				os.Exit(1)
			}

			return ps, nil
		},
		"bzz": func(ctx *adapters.ServiceContext) (node.Service, error) {
			addr := network.NewAddrFromNodeID(ctx.Config.ID)
			params := network.NewHiveParams()
			params.Discovery = false
			config := &network.BzzConfig{
				OverlayAddr:  addr.Over(),
				UnderlayAddr: addr.Under(),
				HiveParams:   params,
			}
			return network.NewBzz(config, kademlia(ctx.Config.ID), stateStore), nil
		},
	}
}

func shutdown(t *testing.T, err error) {
	termbox.Close()
	if err != nil {
		t.Fatalf("%v", err)
	}
}
