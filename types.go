package main

import (
	"time"
)

type chatMsg struct {
	Serial uint64
	Content []byte
	Source string
}

type chatPing struct {
	Created time.Time
	Pong bool
}

type chatAck struct {
	Seen time.Time
	Serial uint64
}

