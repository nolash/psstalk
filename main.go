package main

import (
	"github.com/nolash/psstalk/term"
	"github.com/ethereum/go-ethereum/log"
	"os"
)

var (
	myNick string = "self"
	run bool = true
	freeze bool = false
	chatlog log.Logger
)

func init() {
	hs := log.StreamHandler(os.Stderr, log.TerminalFormat(true))
	hf := log.LvlFilterHandler(log.LvlTrace, hs)
	h := log.CallerFileHandler(hf)
	log.Root().SetHandler(h)
	chatlog = log.New("chatlog", "main")
}

func main() {
	var err error
	//quitC := make(chan struct{})

	client = term.NewTalkClient(2)

	err = startup()
	if err != nil {
		e := newPssTalkError(pssTalkErrorInit)
		chatlog.Crit(e.Error())
		os.Exit(1)
	}

	prompt.Reset()

	chatlog.Debug("ok")

	shutdown()
}


