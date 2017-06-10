package main

import (
	"fmt"
	"math/rand"

	termbox "github.com/nsf/termbox-go"
	"github.com/nolash/psstalk/talk"
)

const (
	connOK = iota
	connDialing
	connP2pDown
	connRPCDown
	connReconn
)

var (
	pssServiceName                        = "pss"
	bzzServiceName                        = "bzz"
	unsentColor         termbox.Attribute = termbox.ColorWhite
	pendingColor        termbox.Attribute = termbox.ColorDefault
	successColor        termbox.Attribute = termbox.ColorGreen
	failColor           termbox.Attribute = termbox.ColorRed
	srcFormat = make(map[*talk.TalkSource]termbox.Attribute)
	colorSrc = map[string]*talk.TalkSource{
		"error": &talk.TalkSource{Nick:string([]byte{0x00, 0x01})},
		"success": &talk.TalkSource{Nick:string([]byte{0x00, 0x01})},
	}
		prompt *Prompt = &Prompt{}
	client *talk.TalkClient
	myFormat termbox.Attribute = termbox.AttrBold | termbox.ColorRed
	bgAttr = termbox.ColorBlack
	bgClearAttr = termbox.ColorBlack
	runeDash rune = 45
	runeSpace rune = 32
)

func init() {
	srcFormat[colorSrc["error"]] = failColor
	srcFormat[colorSrc["success"]] = successColor

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
	src := &talk.TalkSource{
		Nick: nick,
	}
	client.Sources[label] = src
	srcFormat[src] = format
	return nil
}

// get a source from key
func getSrc(label string) *talk.TalkSource {
	return client.Sources[label]
}

// get a random source
func randomSrc() *talk.TalkSource {
	emptysrc := &talk.TalkSource{
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
func updateView(buf *talk.TalkBuffer, startline int, viewportheight int) {
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
func getStartPosition(buf []talk.TalkEntry, viewportheight int) (bufstartline int, bufstartidx int) { 
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
	if i % client.Width > 0 || i == 0 {
		for ; i < client.Width; i++ {
			termbox.SetCell(i%client.Width, prompt.Line+(i/client.Width), runeSpace, bgAttr, bgAttr)
		}
	}
}

