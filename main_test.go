package main

import (
	"io/ioutil"
	"os"
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/nolash/psstalk/talk"
	termbox "github.com/nsf/termbox-go"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/simulations"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/pot"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/pss"
	pssclient "github.com/ethereum/go-ethereum/swarm/pss/client"
	psschat "github.com/nolash/psstalk/protocols/chat"
)

var (
	//services = newServices(psschat.New)
	maxRandomLineLength int
	minRandomLineLength int
	fakenodeconfigs     []*adapters.NodeConfig
	fakepots            = make(map[discover.NodeID]pot.Address)
	outCs               = make(map[discover.NodeID]chan interface{})
	inCs                = make(map[discover.NodeID]chan interface{})
	connCs			= make(map[discover.NodeID]chan *psschat.ChatConn)
)


// initialize the client buffer handler
// draw the mid screen separator
func init() {
	var i int

	client = talk.NewTalkClient(2)
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
		connCs[fakenodeconfig.ID] = make(chan *psschat.ChatConn)
	}

	//adapters.RegisterServices(services)
}

func TestFoo(t *testing.T) {
	t.Fatalf("Only run one of the tests, please")
}

func TestRandomOutput(t *testing.T) {
	var err error
	logC := make(chan []rune)
	chatC := make(chan []rune)
	quitTickC := make(chan struct{})
	quitC := make(chan struct{})

	err = startup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	getLineLengths()

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

	shutdown()
}

func TestInputAndRandomOutput(t *testing.T) {
	var err error
	meC := make(chan []rune)
	otherC := make(chan []rune)
	promptC := make(chan bool)
	quitTickC := make(chan struct{})
	quitC := make(chan struct{})

	err = startup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	getLineLengths()

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

	shutdown()
}

func TestPssReceive(t *testing.T) {
	var err error
	otherC := make(chan []rune)
	quitTickC := make(chan struct{})
	pssInC := inCs[fakenodeconfigs[0].ID]
	pssOutC := outCs[fakenodeconfigs[0].ID]
	pssConnC := connCs[fakenodeconfigs[0].ID]
	quitC := make(chan struct{})
	quitPssC := make(chan struct{})
	txtreplies := []string{
		"hello",
		"here",
		"is",
		"pss!",
	}

	addSrc("only", "bob", termbox.ColorYellow)

	err = startup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	prompt.Reset()

	otherticker := time.NewTicker(time.Millisecond * 500)

	proto := psschat.New(fakenodeconfigs[0].ID[:], "foo", pssInC, pssConnC, newTestChatInject(pssOutC))

	ctx, cancel := context.WithCancel(context.Background())
	_, ps := newPss(t, ctx, cancel, proto, quitPssC)

	pss.RegisterPssProtocol(ps, &psschat.ChatTopic, psschat.ChatProtocol, proto)
	if err != nil {
		t.Fatalf("pss create fail: %v", err)
	}

	// set up the message to send
	code, ok := psschat.ChatProtocol.GetCode(&psschat.ChatMsg{})
	if !ok {
		t.Fatalf("get chatmsg code fail!")
	}
	go func() {
		serial := 1
		for run {
			select {
			case <-otherticker.C:
				payload := psschat.ChatMsg{
					Serial:  uint64(serial),
					Content: txtreplies[(serial-1)%len(txtreplies)],
					Source:  "bar",
				}
				pmsg, err := pss.NewProtocolMsg(code, payload)
				if err != nil {
					t.Fatalf("new protocomsg fail: %v", err)
				}

				env := pss.NewEnvelope(fakepots[fakenodeconfigs[1].ID].Bytes(), psschat.ChatTopic, pmsg)

				ps.Process(&pss.PssMsg{
					To:      fakepots[fakenodeconfigs[0].ID].Bytes(),
					Payload: env,
				})
				serial++
			case msg := <-pssInC:
				chatmsg, ok := msg.(*psschat.ChatMsg)
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
	shutdown()
}

func TestPssSendAndReceive(t *testing.T) {
	var err error
	var potaddr pot.Address
	var serial int = 1
	meC := make(chan []rune)
	otherC := make(chan []rune)
	promptC := make(chan bool)
	pssInC := inCs[fakenodeconfigs[0].ID]
	pssOutC := outCs[fakenodeconfigs[0].ID]
	pssConnC := connCs[fakenodeconfigs[0].ID]
	quitC := make(chan struct{})
	quitPssC := make(chan struct{})

	addSrc(fmt.Sprintf("%x", fakepots[fakenodeconfigs[0].ID].Bytes()[:8]), "bob", termbox.ColorYellow)

	err = startup()
	if err != nil {
		t.Fatalf(err.Error())
	}

	prompt.Reset()


	proto := psschat.New(fakenodeconfigs[0].ID[:], "foo", pssInC, pssConnC, newTestChatInject(pssOutC))

	ctx, cancel := context.WithCancel(context.Background())
	psc, _ := newPss(t, ctx, cancel, proto, quitPssC)

	copy(potaddr[:], fakepots[fakenodeconfigs[0].ID].Bytes())

	psc.AddPssPeer(potaddr, psschat.ChatProtocol)

	go func() {
		for run {
			select {
			case msg := <-pssInC:
				var rs []rune
				var buf *bytes.Buffer
				chatmsg, ok := msg.(*psschat.ChatMsg)
				buf = bytes.NewBufferString(fmt.Sprintf("%d=", chatmsg.Serial))
				for {
					r, n, err := buf.ReadRune()
					if err != nil || n == 0 {
						break
					}
					rs = append(rs, r)
				}

				buf = bytes.NewBufferString(chatmsg.Content)
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

					

					// send the message to ourselves using pssclient
					payload := psschat.ChatMsg{
						Serial:  uint64(serial),
						Content: string(line),
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
	shutdown()
}

func TestCur (t *testing.T) {
//func TestPssSendAndAutoreplyFull(t *testing.T) {

	//var err error

	var conncount int
	var fakenodes []adapters.Node
	fakeclients := make(map[discover.NodeID]*pssclient.Client)

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

	addSrc(fmt.Sprintf("%x", fakepots[fakenodeconfigs[1].ID].Bytes()[:8]), "alice", termbox.ColorCyan)

	startup()

	prompt.Reset()

	ctx, _ := context.WithCancel(context.Background())

	// initialize the network
	services := newServices(fakenodeconfigs[0].ID[:], "foo", inCs[fakenodeconfigs[0].ID], connCs[fakenodeconfigs[0].ID], newTestChatInject(outC))
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
			shutdown()
			t.Fatalf("node create fail: %v", err)
		}
		fakenodes = append(fakenodes, fakenode)

		if err := simnet.Start(fakenode.ID()); err != nil {
			shutdown()
			t.Fatalf("node start fail: %v", err)
		}

		fakerpc, err := fakenode.Client()
		if err != nil {
			shutdown()
			t.Fatalf("rpc err: %v", err)
		}

		fakeclients[cfg.ID], err = pssclient.NewClientWithRPC(ctx, fakerpc)
		if err != nil {
			t.Fatalf(err)
		}
		fakeclients[cfg.ID].Start()
		//fakeclients[cfg.ID].RunProtocol(newTestProtocol(inCs[cfg.ID], outCs[cfg.ID]))
		fakeclients[cfg.ID].RunProtocol(psschat.New(inCs[cfg.ID], nil, newTestChatInject(outCs[cfg.ID])))

		peerevents := make(chan *p2p.PeerEvent)
		peersub, err := fakerpc.Subscribe(ctx, "admin", peerevents, "peerEvents")
		if err != nil {
			shutdown()
			t.Fatalf("node subscribe fail: %v", err)
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

	fakeclients[fakenodeconfigs[0].ID].AddPssPeer(fakepots[fakenodeconfigs[1].ID], psschat.ChatProtocol)
	fakeclients[fakenodeconfigs[1].ID].AddPssPeer(fakepots[fakenodeconfigs[0].ID], psschat.ChatProtocol)

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
						payload := psschat.ChatMsg{
							Serial:  uint64(serialother),
							Content: txtreplies[rand.Int() % len(txtreplies)],
							Source:  fmt.Sprintf("%x", srcpot[:8]),
						}

						outCs[fakenodeconfigs[1].ID] <- payload
						atomic.AddInt32(&serialother, 1)
					}()

				case chatmsg := <-inCs[fakenodeconfigs[0].ID]:
					var rs []rune
					var buf *bytes.Buffer
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

					// send on the outchannel, which writes to the pssclient rw
					payload := psschat.ChatMsg{
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

	shutdown()
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

func newPss(t *testing.T, ctx context.Context, cancel func(), proto *p2p.Protocol, quitC chan struct{}) (*pssclient.Client, *pss.Pss) {
	var err error

	conf := pssclient.NewClientConfig()

	psc, err := pssclient.NewClient(ctx, cancel, conf)
	if err != nil {
		t.Fatalf(err)
	}

	ps := pss.NewTestPss(fakepots[fakenodeconfigs[0].ID].Bytes())

	srv := rpc.NewServer()
	srv.RegisterName("pss", pss.NewAPI(ps))
	srv.RegisterName("psstest", pss.NewAPITest(ps))
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

func getLineLengths() {
	minRandomLineLength = int(float64(client.Width) * 0.4)
	maxRandomLineLength = int(float64(client.Width) * 2.8)
}

func newServices(oaddr []byte, nick string, inC chan interface{}, connC chan *psschat.ChatConn, injectfunc func(*psschat.ChatCtrl) *p2p.Protocol) adapters.Services {
//func newServices(oaddr []byte, nick string, inC chan interface{}, connC chan *psschat.ChatConn, protofunc func(chan *psschat.ChatMsg, chan psschat.ChatConn, func(*psschat.ChatCtrl)) *p2p.Protocol) adapters.Services {
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

			proto := protofunc(inCs[ctx.Config.ID], nil, newTestChatInject(outCs[ctx.Config.ID]))

			pss.RegisterPssProtocol(ps, &psschat.ChatTopic, psschat.ChatProtocol, proto)
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

func newTestChatInject(outC chan interface{}) func (*psschat.ChatCtrl) {
	return func(ctrl *psschat.ChatCtrl) {
		if outC != nil {
			go func() {
				for {
					select {
					case msg := <-outC:
						err := ctrl.Peer.Send(msg)
						if err != nil {
							//pp.Drop(err)
							//break
						}
					}
				}
			}()
		}
	}
}
