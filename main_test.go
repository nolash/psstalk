package main

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
	"unicode/utf8"
	
	termbox "github.com/nsf/termbox-go"
	talk "github.com/nolash/psstalk/client"
)

var (
	unsentColor termbox.Attribute = termbox.ColorWhite
	pendingColor termbox.Attribute = termbox.ColorBlue
	successColor termbox.Attribute = termbox.ColorGreen
	failColor termbox.Attribute = termbox.ColorRed
	maxRandomLineLength int
	minRandomLineLength int
)

// initialize the client buffer handler
// draw the mid screen separator
func init() {
	var err error
	srcFormat = make(map[*talk.TalkSource]termbox.Attribute)
	
	client = talk.NewTalkClient(2)
	
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
			r := randomLine([]rune{int32((i / 10) % 10) + 48, int32(i % 10) + 48, 46, 46, 46, 32})
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
			r := randomLine([]rune{int32((i / 10) % 10) + 48, int32(i % 10) + 48, 46, 46, 46, 32})
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
func randomLine(prefix []rune) (rline []rune) {
	b := make([]byte, 1)
	rlinelen := (rand.Int() % (maxRandomLineLength - minRandomLineLength)) + minRandomLineLength + 1
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
