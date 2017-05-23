package client

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

type TalkClient struct {
	Sources []*TalkSource
	Buffers []*TalkBuffer
	Width int
	Lines []int
}

func NewTalkClient(buffercount int) (c *TalkClient) {
	c = &TalkClient {
		Buffers: make([]*TalkBuffer, buffercount),
		Lines: make([]int, buffercount),
	}
	for i := 0; i < buffercount; i++ {
		c.Buffers[i] = &TalkBuffer{}
	}
	return
}
