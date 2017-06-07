package term

import (
	"bytes"
)

const (
	bufferLines = 1024
)

type TalkSource struct {
	Nick string
}

type TalkEntry struct {
	Source *TalkSource
	Content []rune
}

func (self *TalkEntry) Runes(sep string) (rb []rune) {

	source := ""
	if self.Source != nil {
	//if false {
		source = self.Source.Nick
		if sep == "" {
			sep = ": "
		}
		source += sep

		buf := bytes.NewBufferString(source)
		for {
			r, s, err := buf.ReadRune()
			if err != nil || s == 0 {
				break
			}
			rb = append(rb, r)
		}
	}

	for _, r := range self.Content {
		rb = append(rb, r)
	}
	return rb
}

type TalkBuffer struct {
	Buffer []TalkEntry
	Dirty bool
}

func (self *TalkBuffer) Add(src *TalkSource, line []rune) {
	self.Buffer = append(self.Buffer, TalkEntry{
		Source: src,
		Content: line,
	})
	self.Dirty = true
}

func (self *TalkBuffer) Last() *TalkEntry {
	if len(self.Buffer) == 0 {
		return nil
	}
	return &self.Buffer[len(self.Buffer) - 1]
}

func (self *TalkBuffer) Count() int {
	return len(self.Buffer)
}

type TalkClient struct {
	Sources map[string]*TalkSource
	Buffers []*TalkBuffer
	History []*TalkEntry
	Width int
	Lines []int
	Serial int
}

func NewTalkClient(buffercount int) (c *TalkClient) {
	c = &TalkClient {
		Buffers: make([]*TalkBuffer, buffercount),
		Lines: make([]int, buffercount),
		Sources: make(map[string]*TalkSource),
		Serial: 1,
	}
	for i := 0; i < buffercount; i++ {
		c.Buffers[i] = &TalkBuffer{}
	}
	return
}

