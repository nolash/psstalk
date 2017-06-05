package main

import (
	"fmt"
	"os"
	termbox "github.com/nsf/termbox-go"
	"github.com/nolash/psstalk/term"
)

var (
	myNick string = "self"
	run bool = true
	freeze bool = false
)

// initialize the client buffer handler
// draw the mid screen separator
func init() {
	var err error
	srcFormat = make(map[*term.TalkSource]termbox.Attribute)
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
}

func main() {
	//var err error
	quitC := make(chan struct{})

	for run {
		ev := termbox.PollEvent()
		if ev.Type == termbox.EventKey {
			if freeze {
				quitC <- struct{}{}
				break
			} else {
				freeze = true
			}
		}
	}

	termbox.Close()
}


