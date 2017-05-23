package main

import (
	"math/rand"
	termbox "github.com/nsf/termbox-go"
	talk "github.com/nolash/psstalk/client"
)

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
	client *talk.TalkClient
	myFormat termbox.Attribute = termbox.AttrBold | termbox.ColorRed
	srcFormat map[*talk.TalkSource]termbox.Attribute
	bgAttr = termbox.ColorBlack
	bgClearAttr = termbox.ColorBlue
	runeDash rune = 45
	runeSpace rune = 32
)

// add a chat source (peer)
func addSrc(nick string, format termbox.Attribute) error {
	src := &talk.TalkSource{
		Nick: nick,
	}
	client.Sources = append(client.Sources, src)
	srcFormat[src] = format
	return nil
}

// get a random source
func randomSrc() *talk.TalkSource {
	if len(client.Sources) == 0 {
		return nil
	}
	return client.Sources[rand.Int() % len(client.Sources)]
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
		for ii, r = range buf.Buffer[i].Content {
			if ii < skip {
				continue
			}
			termbox.SetCell((ii - skip) % client.Width, startline + lines + ((ii - skip) / client.Width), r, srcFormat[buf.Buffer[i].Source], bgAttr)
		}
		lines += ((ii - skip) / client.Width) + 1
		partialfill := len(buf.Buffer[i].Content) % client.Width
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

