package term

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

func (self *TalkEntry) Runes(sep string) (r []rune) {
	if sep == "" {
		sep = ": "
	}
	for _, b := range self.Source.Nick {
		r = append(r, int32(b))
	}
	for _, b := range sep {
		r = append(r, int32(b))
	}
	for _, b := range self.Content {
		r = append(r, b)
	}
	return r
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
