package term

import (
	"encoding/hex"
	"bytes"
	"fmt"
	"strings"
)

const (
	bufferLines = 1024
)

var (
	cmdAll = uint32(1)
	cmdAdd = uint32(2)
	cmdMsg = uint32(4)
	cmdDone = uint32(0x80000000)
	cmdDefault = cmdAll
)

type TalkSource struct {
	Nick string
	Addr []byte
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
	action uint32
	arguments []string
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

// result: string result describing the result of the command
// payload: little-endian packed byte for network transport of content
// err: set if anything went wrong
func (self *TalkClient) Process(line []rune) (result string, payload string, err error) {
	var c int
	processed := false
	input := string(line)

	// short circuit if no command
	if strings.Index(input, "/") != 0 {
		self.action = cmdDefault
		return "", input, nil
	}

	// ditch the leading slash
	input = input[1:]

	// split on spaces
	// process till end of commands
	for !processed {
		var done bool
		c = strings.Index(input, " ")
		if c == -1 {
			c = len(input)
			processed = true
		}
		done, result, err = self.addCmd(input[:c])
		if err != nil {
			return "invalid command", "", err
		}

		if !processed {
			c++
		}

		input = input[c:]

		if done {
			break
		}
	}

	if !self.readyCmd() {
		self.ResetCmd()
		return "incomplete command", "", fmt.Errorf("%v", input)
	}

	return result, input, nil
}

func (self *TalkClient) IsSendCmd() bool {
	return self.action & (cmdMsg | cmdAll) > 0
}

func (self *TalkClient) IsAddCmd() bool {
	return self.action & cmdAdd > 0
}

func (self *TalkClient) GetCmd() []string {
	if !self.readyCmd() {
		return nil
	}
	return self.arguments
}

func (self *TalkClient) readyCmd() bool {
	return int32(self.action) < 0
}

func (self *TalkClient) ResetCmd() {
	self.action = 0x0
	self.arguments = []string{}
}

func (self *TalkClient) addCmd(cmd string) (bool, string, error) {
	result := ""
	if self.action < 0 {
		return true, result, nil
	}

	switch self.action {
		case cmdAdd:
			self.arguments = append(self.arguments, cmd)
			if len(self.arguments) == 2 {
				addr, err := hex.DecodeString(self.arguments[1])
				if err != nil {
					self.ResetCmd()
					return false, result, fmt.Errorf("cannot parse address to add")
				}

				for _, src := range self.Sources {
					fmt.Printf("checking %v %v\n", addr, src.Addr)
					if bytes.Equal(addr, src.Addr) {
						self.ResetCmd()
						return false, result, fmt.Errorf("address already added")
					}
				}
				src := &TalkSource{
					Nick: self.arguments[0],
					Addr: addr,
				}

				self.Sources[self.arguments[0]] = src
				result = fmt.Sprintf("ok added %x as '%s'", src.Addr, src.Nick)
				self.action |= cmdDone
			}
		default:
			switch cmd {
				case "add":
					self.action = cmdAdd
				default:
					return false, result, fmt.Errorf("unknown command")
			}
	}
	if int32(self.action) < 0 {
		return true, result, nil
	}
	return false, result, nil
}

// little endian packing for transport
//func packRunes(line []rune) []byte {
//	var b []byte
//	rb := bytes.NewReader(b)
//	wb = bytes.NewBuffer(payload)
//	for i = 0; i < len(b); {
//		four := make([]byte, 4)
//		r, s, err := rb.ReadRune()
//		if err != nil {
//			return nil, nil, fmt.Errorf("unexpected error in packing argument")
//		}
//		binary.LittleEndian.PutUint32(four, uint32(r))
//		wb.Write(four)
//
//		i += s
//	}
//	//payload = wb.Bytes()
//	return
//}
//
