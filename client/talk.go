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
	Chat *TalkBuffer
	Log *TalkBuffer
}

func NewTalkClient() (c *TalkClient) {
	c = &TalkClient {
		Chat: &TalkBuffer{},
		Log: &TalkBuffer{},
	}
	return
}

