package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"
	"unicode/utf8"
	
	termbox "github.com/nsf/termbox-go"
	talk "github.com/nolash/psstalk/client"
)

var (
	myNick = "self"
	myFormat = termbox.AttrBold | termbox.ColorRed
	srcFormat map[*talk.TalkSource]termbox.Attribute
	bgAttr = termbox.ColorBlack
	bgClearAttr = termbox.ColorBlue
	client *talk.TalkClient
	run bool = true
	freeze bool = false
	runeDash rune = 45
	runeSpace rune = 32
	viewWidth int
	viewLogHeight int
	viewChatHeight int
	maxRandomLineLength int
	minRandomLineLength int
)

// initialize the client buffer handler
// draw the mid screen separator
func init() {
	var err error
	srcFormat = make(map[*talk.TalkSource]termbox.Attribute)
	client = talk.NewTalkClient()
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
	for i := 0; i < viewWidth; i++ {
		termbox.SetCell(i, viewChatHeight, runeDash, termbox.ColorYellow, termbox.ColorBlack)
	}
	termbox.Flush()
}


func main() {
	//var err error
	logC := make(chan []rune)
	//chatC := make(chan []rune)
	quitTickC := make(chan struct{})
	quitC := make(chan struct{})
	
	addSrc("bob", termbox.ColorYellow)
	addSrc("alice", termbox.ColorGreen)

	ticker := time.NewTicker(time.Second)
	rand.Seed(time.Now().Unix())
	
	go func() {
		for i := 0; run; i++ {
			select {
				case <- ticker.C:
					r := randomLine([]rune{int32((i / 10) % 10) + 48, int32(i % 10) + 48, 46, 46, 46, 32})
					client.Log.Add(nil, r)
					logC <- r
				case <- quitTickC:
					ticker.Stop()
					break
			}
		}	
	}()
	
	go func() {
		for run {
			select {
				case <- logC:
					updateView(client.Log, viewChatHeight + 1, &viewLogHeight)
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


// add a chat source (peer)
func addSrc(nick string, format termbox.Attribute) error {
	src := &talk.TalkSource{
		Nick: nick,
	}
	client.Sources = append(client.Sources, src)
	srcFormat[src] = format
	return nil
}

// startline: termbox line to start refresh from
// height: height of termbox viewport for buffer
func updateView(buf *talk.TalkBuffer, startline int, viewportheight *int) {
	var skip int
	lines := 0
	bufline, skip := getStartPosition(buf.Buffer, *viewportheight)
	
	for i := bufline; i < len(buf.Buffer); i++ {
		var ii int
		var r rune
		for ii, r = range buf.Buffer[i].Content {
			if ii < skip {
				continue
			}
			termbox.SetCell((ii - skip) % viewWidth, startline + lines + ((ii - skip) / viewWidth), r, srcFormat[buf.Buffer[i].Source], bgAttr)
		}
		lines += ((ii - skip) / viewWidth) + 1
		partialfill := len(buf.Buffer[i].Content) % viewWidth
		for ; partialfill < viewWidth; partialfill++ {
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
			bufstartidx = ((lines - viewportheight) * viewWidth)
			break
		}
	}
	return
}

// find the vertical middle
func updateSize() {
	w, h := termbox.Size()
	viewWidth = w
	viewChatHeight = h / 2
	viewLogHeight = viewChatHeight
	if (viewChatHeight / 2) * 2 == viewChatHeight {
		viewLogHeight--
	}
	minRandomLineLength = int(float64(w) * 1.25)
	maxRandomLineLength = int(float64(w) * 2.75)
}

// how many rows does the line span
// todo: wrap at previous whitespace if last word extends width
func lineRows(rline []rune) int {
	return (len(rline) / viewWidth) + 1
}

// generate a random line of content
func randomLine(prefix []rune) (rline []rune) {
	b := make([]byte, 1)
	rlinelen := (rand.Int() % (maxRandomLineLength - minRandomLineLength)) + minRandomLineLength + 1
	//rlinelen := int(float64(viewWidth) * 1.5)
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
