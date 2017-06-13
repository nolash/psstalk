package chat

import (
	//"bytes"
	"fmt"
	"time"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/protocols"
	"github.com/ethereum/go-ethereum/swarm/pss"
	"github.com/ethereum/go-ethereum/log"
)

const (
	EPing = iota
	EHandshake
	ESendFail
)

var (
	chatConnString = map[int]string{
		EPing: "Ping",
		ESendFail: "Send error",
		EHandshake: "Handshake",
	}
)

type ChatMsg struct {
	Serial uint64
	Content string
	Source string
}

type ChatPing struct {
	Addr []byte
	Pong bool
}

type ChatAck struct {
	Seen time.Time
	Serial uint64
}

type ChatHandshake struct {
	Addr []byte
	Nick string
}

var ChatProtocol = &protocols.Spec{
	Name: "pssChat",
	Version: 1,
	MaxMsgSize: 1024,
	Messages: []interface{}{
		ChatMsg{}, ChatPing{}, ChatAck{}, ChatHandshake{},
	},
}

var ChatTopic = pss.NewTopic(ChatProtocol.Name, int(ChatProtocol.Version))

type ChatConn struct {
	Addr []byte
	E int
	Detail string
}

func (c* ChatConn) Error() string {
	return chatConnString[c.E]
}

type ChatCtrl struct {
	Peer *protocols.Peer
	PeerOAddr []byte
	ConnC chan *ChatConn
	InC chan interface{}
	OAddr []byte

	nick string
	//pingTX int
	//pingRX int
	//pingLast int
	injectfunc func(*ChatCtrl)
}

func (self *ChatCtrl) chatHandler(msg interface{}) error {
	chatmsg, ok := msg.(*ChatMsg)
	if ok {
		if self.InC != nil {
			self.InC <- chatmsg
		}

		log.Debug("sending ack", "peer", self.Peer.ID(), "msgserial", chatmsg.Serial)
		self.Peer.Send(&ChatAck{
			Seen: time.Now(),
			Serial: chatmsg.Serial,
		})
		return nil
	}

	chatping, ok := msg.(*ChatPing)
	if ok {
		if !chatping.Pong {
			log.Debug("sending pong", "peer", self.Peer.ID())
			self.Peer.Send(&ChatPing{
				Pong: true,
			})
			self.ConnC <- &ChatConn{
				Addr: chatping.Addr,
				E: EPing,
			}
		} else {
			log.Debug("got pong", "peer", self.Peer.ID())
		}
		return nil
	}

	chatack, ok := msg.(*ChatAck)
	if ok {
		log.Debug("got ack", "msgserial", chatack.Serial, "msgseen", chatack.Seen)
		if self.InC != nil {
			self.InC <- chatack
		}
		return nil
	}

	chaths, ok := msg.(*ChatHandshake)
	if ok {
		log.Debug("processing handshake", "from", chaths.Nick)
		self.PeerOAddr = chaths.Addr
		self.ConnC <- &ChatConn{
			Addr: chaths.Addr,
			E: EHandshake,
			Detail: chaths.Nick,
		}
		self.injectfunc(self)
		return nil
	}

	return fmt.Errorf("invalid psschat protocol message")
}

func New(oaddr []byte, nick string, inC chan interface{}, connC chan *ChatConn, injectfunc func(*ChatCtrl)) *p2p.Protocol {
	chatctrl := &ChatCtrl{
		InC: inC,
		ConnC: connC,
		OAddr: oaddr,
		injectfunc: injectfunc,
	}

	if nick == "" {
		nick = fmt.Sprintf("%x", oaddr[:4])
	}

	chatctrl.nick = nick

	return &p2p.Protocol{
		Name:    ChatProtocol.Name,
		Version: ChatProtocol.Version,
		Length:  3,
		Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
			pp := protocols.NewPeer(p, rw, ChatProtocol)
			chatctrl.Peer = pp
			//injectfunc(chatctrl)
			go func() {
				time.Sleep(time.Microsecond * 100)
				err := pp.Send(&ChatHandshake{
					Addr: chatctrl.OAddr,
					Nick: chatctrl.nick,
				})
				if err != nil {
					log.Error("Failed sending handshake, so other side will have to add manually")
				}
			}()
			pp.Run(chatctrl.chatHandler)
			return nil
		},
	}
}
