package talk

import (
	"encoding/hex"
	"bytes"
	"fmt"
	"strings"
	"time"
)

const (
	bufferLines = 1024
)

var (
	cmdAll = uint32(1)
	cmdAdd = uint32(1 << 1)
	cmdDel = uint32(1 << 2)
	cmdMsg = uint32(1 << 8)
	cmdDone = uint32(0x80000000)
	cmdDefault = cmdAll
)

type TalkAddress []byte

type TalkSource struct {
	Nick string
	Addr TalkAddress
	Seen time.Time
}

type TalkEntry struct {
	Source *TalkSource
	Content []rune
	Id int // enables matching of line to message serial (or other desired id) after merge in viewport buffer
}

func (self *TalkEntry) Runes(sep string) (rb []rune) {

	if self.Source != nil {
		source := self.Source.Nick
		if source[0] != 0x00 {
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
	self.ResetCmd()

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

	if self.IsSendCmd() {
		if  input == "" {
			self.ResetCmd()
			return "incomplete command", "", fmt.Errorf("send command without content")
		}
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
			var src *TalkSource
			self.arguments = append(self.arguments, cmd)
			if len(self.arguments) == 2 {
				if self.Sources[self.arguments[0]] != nil {
					self.ResetCmd()
					return false, result, fmt.Errorf("handle already in use")
				}
				addr, err := StringToAddress(cmd)
				if err != nil {
					self.ResetCmd()
					return false, result, err
				}
				src = self.getSourceByAddress(addr)
				if src != nil {
					self.ResetCmd()
					return false, result, fmt.Errorf("address already added")
				}
				src = &TalkSource{
					Nick: self.arguments[0],
					Addr: addr,
				}

				self.Sources[self.arguments[0]] = src
				result = fmt.Sprintf("ok added %x as '%s'", src.Addr, src.Nick)
				self.action |= cmdDone
			}
		case cmdMsg:
			var src *TalkSource
			self.arguments = append(self.arguments, cmd)
			if src = self.Sources[cmd]; src == nil {
				if src = self.getSourceByNick(cmd); src == nil {
					addr, err := StringToAddress(cmd)
					if err != nil {
						self.ResetCmd()
						return false, result, err
					}
					if self.getSourceByAddress(addr) == nil {
						self.ResetCmd()
						return false, result, fmt.Errorf("no recipient by that name or address")
					}
				} else {
					cmd = AddressToString(src.Addr)
				}
			} else {
				cmd = AddressToString(src.Addr)
			}
			self.arguments[0] = cmd
			self.action |= cmdDone

		case cmdDel:
			var src *TalkSource
			if src = self.Sources[cmd]; src == nil {
				self.ResetCmd()
				return false, result, fmt.Errorf("no handle by that name")
			}
			delete(self.Sources, cmd)
			self.action |= cmdDone
		default:
			switch cmd {
				case "add":
					self.action = cmdAdd
				case "msg":
					self.action = cmdMsg
				case "del":
					self.action = cmdDel
				case "rm":
					self.action = cmdDel
				default:
					return false, result, fmt.Errorf("unknown command")
			}
	}
	if int32(self.action) < 0 {
		return true, result, nil
	}
	return false, result, nil
}

func AddressToString(addr TalkAddress) string {
	return bytes.NewBuffer(addr).String()
}

func StringToAddress(straddr string) (TalkAddress, error) {
	addr, err := hex.DecodeString(straddr)
	if err != nil {
		return nil, fmt.Errorf("cannot parse address to add")
	}
	return addr, nil
}

func (self *TalkClient) getSourceKey(src *TalkSource) string {
	for k, v := range self.Sources {
		if v == src {
			return k
		}
	}
	return ""
}

func (self *TalkClient) getSourceByNick(nick string) *TalkSource {
	for _, src := range self.Sources {
		if src.Nick == nick {
			return src
		}
	}
	return nil
}

func (self *TalkClient) getSourceByAddress(addr TalkAddress) *TalkSource {
	for _, src := range self.Sources {
		if bytes.Equal(addr, src.Addr) {
			return src
		}
	}
	return nil
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
