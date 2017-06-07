package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	//"time"

	termbox "github.com/nsf/termbox-go"
	"github.com/nolash/psstalk/term"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/swarm/pss"
	"github.com/ethereum/go-ethereum/p2p/simulations/adapters"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/swarm/network"
	"github.com/ethereum/go-ethereum/swarm/storage"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
)

var (
	pssServiceName                        = "pss"
	bzzServiceName                        = "bzz"
	unsentColor         termbox.Attribute = termbox.ColorWhite
	pendingColor        termbox.Attribute = termbox.ColorDefault
	successColor        termbox.Attribute = termbox.ColorGreen
	failColor           termbox.Attribute = termbox.ColorRed
	srcFormat = make(map[*term.TalkSource]termbox.Attribute)
	prompt *Prompt = &Prompt{}
	client *term.TalkClient
	myFormat termbox.Attribute = termbox.AttrBold | termbox.ColorRed
	bgAttr = termbox.ColorBlack
	bgClearAttr = termbox.ColorBlack
	runeDash rune = 45
	runeSpace rune = 32

)


var chatProtocol = &protocols.Spec{
	Name: "psschat",
	Version: 1,
	MaxMsgSize: 1024,
	Messages: []interface{}{
		chatMsg{}, chatPing{}, chatAck{},
	},
}

var chatTopic = pss.NewTopic(chatProtocol.Name, int(chatProtocol.Version))

type chatCtrl struct {
	inC chan interface{}
	peer *protocols.Peer
	oAddr []byte
}

func (self *chatCtrl) chatHandler(msg interface{}) error {
	if self.inC != nil {
		self.inC <- msg
	}
	/*self.peer.Send(chatMsg{
		Source: fmt.Sprintf("%x",
		Content: txtreplies[rand.Int() % len(txtreplies)],
	}*/
	return nil
}

type Prompt struct {
	Buffer []rune
	Count int
	Line int
}

func (self *Prompt) Reset() {
	self.Buffer = []rune{}
	self.Count = 0
}

func (self *Prompt) Add(r rune) {
	self.Buffer = append(self.Buffer, r)
	self.Count++
}

func (self *Prompt) Remove() {
	self.Buffer = self.Buffer[0:self.Count - 1]
	self.Count--
}

// add a chat source (peer)
func addSrc(label string, nick string, format termbox.Attribute) error {
	src := &term.TalkSource{
		Nick: nick,
	}
	client.Sources[label] = src
	srcFormat[src] = format
	return nil
}

// get a source from key
func getSrc(label string) *term.TalkSource {
	return client.Sources[label]
}

// get a random source
func randomSrc() *term.TalkSource {
	emptysrc := &term.TalkSource{
		Nick: "noone",
	}
	if len(client.Sources) == 0 {
		return emptysrc
	}
	idx := rand.Int() % len(client.Sources)
	i := 0
	for _, s := range client.Sources {
		if i == idx {
			return s
		}
		i++
	}
	return emptysrc
}

func startup() error {
	var err error
	err = termbox.Init()
	if err != nil {
		panic("could not init termbox")
	}
	err = termbox.Clear(termbox.ColorYellow, termbox.ColorBlack)
	if err != nil {
		shutdown()
		return fmt.Errorf("Couldn't clear screen: %v", err)
	}
	updateSize()
	for i := 0; i < client.Width; i++ {
		termbox.SetCell(i, client.Lines[0], runeDash, termbox.ColorYellow, termbox.ColorBlack)
	}
	termbox.Flush()
	return nil
}

func shutdown() {
	termbox.Close()
}

// startline: termbox line to start refresh from
// viewportheight: height of termbox viewport for buffer
func updateView(buf *term.TalkBuffer, startline int, viewportheight int) {
	var skip int
	lines := 0
	bufline, skip := getStartPosition(buf.Buffer, viewportheight)

	for i := bufline; i < len(buf.Buffer); i++ {
		var ii int
		var r rune

		content := buf.Buffer[i].Runes("")
		for ii, r = range content {
			if ii < skip {
				continue
			}
			termbox.SetCell((ii - skip) % client.Width, startline + lines + ((ii - skip) / client.Width), r, srcFormat[buf.Buffer[i].Source], bgAttr)
		}
		lines += ((ii - skip) / client.Width) + 1
		partialfill := len(content) % client.Width
		for ; partialfill < client.Width; partialfill++ {
			termbox.SetCell(partialfill, startline + lines - 1, runeSpace, bgAttr, bgClearAttr)
		}
		skip = 0
	}
}

// work lines (buffer entries) backwards from end of buffer till viewport height threshold is reached 
// within the line, find the cell index of the row thats on the threshold
func getStartPosition(buf []term.TalkEntry, viewportheight int) (bufstartline int, bufstartidx int) { 
	lines := 0
	for bufstartline = len(buf); bufstartline > 0; bufstartline-- {
		currentlines := lineRows(buf[bufstartline - 1].Content)
		lines += currentlines
		if lines >= viewportheight {
			bufstartline--
			bufstartidx = ((lines - viewportheight) * client.Width)
			break
		}
	}
	return
}


// how many rows does the line span
// todo: wrap at previous whitespace if last word extends width
func lineRows(rline []rune) int {
	return (len(rline) / client.Width) + 1
}

func addToPrompt(r rune, before int) {
	prompt.Add(r)
	// if the line count changes also update the message buffer, less the lines that the prompt buffer occupies
	promptlines := prompt.Count / client.Width
	if prompt.Count/client.Width > before {
		viewlines := 0
		for _, entry := range client.Buffers[0].Buffer {
			viewlines += lineRows(entry.Content)
			if viewlines >= client.Lines[0] {
				prompt.Line--
				for i := 0; i < client.Width; i++ {
					termbox.SetCell(i, prompt.Line+(prompt.Count/client.Width), runeSpace, bgAttr, bgAttr)
				}
				break
			}
		}
		updateView(client.Buffers[0], 0, client.Lines[0]-(promptlines+1)-1)
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
		updateView(client.Buffers[0], 0, client.Lines[0]-(now+1))
	}
	updatePromptView()
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
}

func updatePromptView() {
	var i int
	//lines := prompt.Line + (prompt.Count / client.Width)

	// write buffer to terminal at prompt position
	for i = 0; i < prompt.Count; i++ {
		termbox.SetCell(i%client.Width, prompt.Line+(i/client.Width), prompt.Buffer[i], unsentColor, bgAttr)
	}

	// clear remaining lines
	if i%client.Width > 0 {
		for ; i < client.Width; i++ {
			termbox.SetCell(i%client.Width, prompt.Line+(i/client.Width), runeSpace, bgAttr, bgAttr)
		}
	}
}

func newServices(protofunc func(chan interface{}, chan interface{}) *p2p.Protocol) adapters.Services {
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

			proto := protofunc(nil, nil)

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

