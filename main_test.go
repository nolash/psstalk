package main

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
	"unicode/utf8"

	termbox "github.com/nsf/termbox-go"
	"github.com/nolash/psstalk/term"

	pssclient "github.com/ethereum/go-ethereum/swarm/pss/client"
	"github.com/ethereum/go-ethereum/swarm/pss"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/pot"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/protocols"
)

var (
	unsentColor termbox.Attribute = termbox.ColorWhite
	pendingColor termbox.Attribute = termbox.ColorDefault
	successColor termbox.Attribute = termbox.ColorGreen
	failColor termbox.Attribute = termbox.ColorRed
	maxRandomLineLength int
	minRandomLineLength int
	chatlog log.Logger
	fakeselfaddr = network.RandomAddr().OAddr
	fakeotheraddr = network.RandomAddr().OAddr
)

// initialize the client buffer handler
// draw the mid screen separator
func init() {

	client = term.NewTalkClient(2)

	err := termbox.Init()
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

}


func TestRandomOutput(t *testing.T) {
	//var err error
	logC := make(chan []rune)
	chatC := make(chan []rune)
	quitTickC := make(chan struct{})
	quitC := make(chan struct{})

	

	rand.Seed(time.Now().Unix())

	addSrc("bob", termbox.ColorYellow)
	addSrc("alice", termbox.ColorGreen)

	logticker := time.NewTicker(time.Millisecond * 250)
	chatticker := time.NewTicker(time.Millisecond * 600)

	go func() {
		for i := 0; run; i++ {
			r := randomLine([]rune{int32((i / 10) % 10) + 48, int32(i % 10) + 48, 46, 46, 46, 32}, 0)
			select {
				case <- chatticker.C:
					client.Buffers[0].Add(randomSrc(), r)
					chatC <- r
				case <- logticker.C:
					client.Buffers[1].Add(nil, r)
					logC <- r
				case <- quitTickC:
					logticker.Stop()
					chatticker.Stop()
					break
			}
		}
	}()

	go func() {
		for run {
			select {
				case <- chatC:
					updateView(client.Buffers[0], 0, client.Lines[0])
					termbox.Flush()
				case <- logC:
					updateView(client.Buffers[1], client.Lines[0] + 1, client.Lines[1])
					termbox.Flush()
				case <- quitC:
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

	termbox.Close()
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
			r := randomLine([]rune{int32((i / 10) % 10) + 48, int32(i % 10) + 48, 46, 46, 46, 32}, 0)
			select {
				case <- otherticker.C:
					client.Buffers[1].Add(randomSrc(), r)
					otherC <- r
				case <- quitTickC:
					otherticker.Stop()
					break
			}
		}
	}()

	go func() {
		for run {
			select {
				case <- meC:
					updateView(client.Buffers[0], 0, client.Lines[0] - 1)
					termbox.Flush()
				case <- otherC:
					updateView(client.Buffers[1], client.Lines[0] + 1, client.Lines[1])
					termbox.Flush()
				case <- promptC:
					termbox.SetCursor(prompt.Count % client.Width, prompt.Line + (prompt.Count / client.Width))
					termbox.Flush()
				case <- quitC:
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
				switch (ev.Key) {
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
						if prompt.Line > client.Lines[0] - 1 {
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

	termbox.Close()
}

func TestPssReceive(t *testing.T) {
	var err error
	otherC := make(chan []rune)
	quitTickC := make(chan struct{})
	pssInC := make(chan interface{})
	pssOutC := make(chan interface{})
	quitC := make(chan struct{})
	quitPssC := make(chan struct{})
	msgs := [][]byte{
		[]byte("hello"),
		[]byte("here"),
		[]byte("is"),
		[]byte("pss!"),
	}

	

	addSrc("bob", termbox.ColorYellow)

	prompt.Reset()

	rand.Seed(time.Now().Unix())

	otherticker := time.NewTicker(time.Millisecond * 500)

	proto  := newProtocol(pssInC, pssOutC)

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
				case <- otherticker.C:
					payload := chatMsg{
						Serial: uint64(serial),
						Content: msgs[(serial - 1) % len(msgs)],
						Source: "bar",
					}
					pmsg, err := pss.NewProtocolMsg(code, payload)
					if err != nil {
					t.Fatalf("new protocomsg fail: %v", err)
					}

					env := pss.NewPssEnvelope(fakeotheraddr, chatTopic, pmsg)

					ps.Process(&pss.PssMsg{
						To: fakeselfaddr,
						Payload: env,
					})
					serial++
				case msg := <-pssInC:
					chatmsg, ok := msg.(*chatMsg)
					if !ok {
						chatlog.Crit("Could not parse chatmsg", "msg", msg)
						quitC<-struct{}{}
					}
					r := []rune{int32(chatmsg.Serial + 0x30), int32(0x3D)}
					for _, b := range chatmsg.Content {
						r = append(r, int32(b))
					}
					client.Buffers[1].Add(randomSrc(), r)
					otherC <- r
					if chatmsg.Serial > 9 {
						quitC<-struct{}{}
					}
				case <- quitTickC:
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
				case <- otherC:
					updateView(client.Buffers[1], client.Lines[0] + 1, client.Lines[1])
					termbox.Flush()
			}
		}
	}()
	<-quitC
	quitTickC <- struct{}{}
	termbox.Close()
}

func TestPssSendAndReceive(t *testing.T) {
	var potaddr pot.Address
	var serial int = 1
	meC := make(chan []rune)
	otherC := make(chan []rune)
	promptC := make(chan bool)
	pssInC := make(chan interface{})
	pssOutC := make(chan interface{})
	quitC := make(chan struct{})
	quitPssC := make(chan struct{})

	

	addSrc(fmt.Sprintf("%x", fakeselfaddr[:4]), termbox.ColorYellow)

	prompt.Reset()

	rand.Seed(time.Now().Unix())

	proto := newProtocol(pssInC, pssOutC)

	ctx, cancel := context.WithCancel(context.Background())
	psc, _ := newPss(t, ctx, cancel, proto, quitPssC)

	copy(potaddr[:], fakeselfaddr)

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
						quitC<-struct{}{}
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
				case <- meC:
					updateView(client.Buffers[0], 0, client.Lines[0] - 1)
					termbox.Flush()
				case <- promptC:
					termbox.SetCursor(prompt.Count % client.Width, prompt.Line + (prompt.Count / client.Width))
					termbox.Flush()
				case <- otherC:
					updateView(client.Buffers[1], client.Lines[0] + 1, client.Lines[1])
					termbox.Flush()
				case <- quitC:
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
				switch (ev.Key) {
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
						if prompt.Line > client.Lines[0] - 1 {
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
							Serial: uint64(serial),
							Content: buf.Bytes(),
							Source: client.Sources[0].Nick,
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
	termbox.Close()
}


// split the screen vertically in two
func updateSize() {
	var h int
	client.Width, h = termbox.Size()
	client.Lines[0] = h / 2
	client.Lines[1] = client.Lines[0]
	if (client.Lines[0] / 2) * 2 == client.Lines[0] {
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
		b[0] = b[0] % 26 + 40
		r, _ := utf8.DecodeRune(b)
		rline = append(rline, r)
	}
	if prefix != nil && len(prefix) <= len(rline) {
		copy(rline[:len(prefix)], prefix[:])
	}
	return
}

func addToPrompt(r rune, before int) {
	prompt.Add(r)
	// if the line count changes also update the message buffer, less the lines that the prompt buffer occupies
	promptlines := prompt.Count / client.Width
	if prompt.Count / client.Width > before {
		viewlines := 0
		for _, entry := range client.Buffers[0].Buffer {
			viewlines += lineRows(entry.Content)
			if viewlines >= client.Lines[0] {
				prompt.Line--
				for i := 0; i < client.Width; i++ {
					termbox.SetCell(i, prompt.Line + (prompt.Count / client.Width), runeSpace, bgAttr, bgAttr)
				}
				break
			}
		}
		updateView(client.Buffers[0], 0, client.Lines[0] - (promptlines + 1) - 1)
	}
	updatePromptView()
}

func removeFromPrompt(before int) {
	if prompt.Count == 0 {
		return
	}
	prompt.Remove()
	now := prompt.Count / client.Width
	if now < before {
		viewlines := 0
		for _, entry := range client.Buffers[0].Buffer {
			viewlines += lineRows(entry.Content)
			if viewlines > client.Lines[0] {
				prompt.Line++
				break
			}
		}
		updateView(client.Buffers[0], 0, client.Lines[0] - (now + 1))
	}
	updatePromptView()
}

func updatePromptView() {
	var i int
	//lines := prompt.Line + (prompt.Count / client.Width)

	// write buffer to terminal at prompt position
	for i = 0; i < prompt.Count; i++ {
		termbox.SetCell(i % client.Width, prompt.Line + (i / client.Width), prompt.Buffer[i], unsentColor, bgAttr)
	}

	// clear remaining lines
	if  i % client.Width > 0 {
		for ; i < client.Width; i++ {
			termbox.SetCell(i % client.Width, prompt.Line + (i / client.Width), runeSpace, bgAttr, bgAttr)
		}
	}
}

func newPss(t *testing.T, ctx context.Context, cancel func(), proto *p2p.Protocol, quitC chan struct{}) (*pssclient.PssClient, *pss.Pss) {
	var err error

	conf := pssclient.NewPssClientConfig()

	psc := pssclient.NewPssClient(ctx, cancel, conf)

	ps := pss.NewTestPss(fakeselfaddr)

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
	return &p2p.Protocol {
		Name: chatProtocol.Name,
		Version: chatProtocol.Version,
		Length: 3,
		Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
			pp := protocols.NewPeer(p, rw, chatProtocol)
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

			pp.Run(chatctrl.chatHandler)
			return nil
		},
	}
}

