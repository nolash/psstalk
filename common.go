package main

import (
	"math/rand"
	//"time"

	termbox "github.com/nsf/termbox-go"
	"github.com/nolash/psstalk/term"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/swarm/pss"
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


var (
	prompt *Prompt = &Prompt{}
	client *term.TalkClient
	myFormat termbox.Attribute = termbox.AttrBold | termbox.ColorRed
	srcFormat map[*term.TalkSource]termbox.Attribute
	bgAttr = termbox.ColorBlack
	bgClearAttr = termbox.ColorBlack
	runeDash rune = 45
	runeSpace rune = 32
)

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


