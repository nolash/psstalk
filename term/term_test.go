package term

import (
	"bytes"
	"testing"
	"flag"
)

const (
	content = "lorem ipsum dolor"
)

var (
	debugflag bool
	client *TalkClient
)

func init() {
	flag.BoolVar(&debugflag, "v", false, "debug output")
	client = NewTalkClient(1)
}

func Test_Process(t *testing.T) {
	var err error
	var payload string
	var i int

	checkbytes := make(map[string][]byte)

	err, _ = process(t, []rune(content))
	if err != nil {
		t.Fatalf("error thrown without command")
	}

	err, _ = process(t, []rune("/foo bar"))
	if err == nil {
		t.Fatalf("invalid command doesn't throw error")
	}

	// for fail on surplus args, see the add test below

	// add tests
	err, _ = process(t, []rune("/add foo"))
	if err == nil {
		t.Fatalf("[add] passes with only one command")
	}

	err, _ = process(t, []rune("/add foo abcdefgh"))
	if err == nil {
		t.Fatalf("[add] allows invalid hex string")
	}

	err, _ = process(t, []rune("/add foo deadbeef"))
	if err != nil {
		t.Fatalf("[add] add with valid handle and hex fails")
	}
	i = len(client.Sources)
	if i != 1 {
		t.Fatalf("[add] source not added to client, expected 1 source, have %d", i)
	}

	err, _ = process(t, []rune("/add bar deadbeef"))
	if err == nil {
		t.Fatalf("[add] allows duplicate addresses")
	}

	err, payload = process(t, []rune("/add bar f00b47 " + content))
	if err != nil {
		t.Fatalf("[add] fails on surplus commands")
	}
	if payload != content {
		t.Fatalf("[add] surplus arguments should be %q, got %q", content, payload)
	}
	i = len(client.Sources)
	if i != 2 {
		t.Fatalf("should have 2 sources, have %d", i)
	}
	checkbytes["bar"] = []byte{0xf0, 0x0b, 0x47}

	// msg

	err, payload = process(t, []rune("/msg foo"))
	if err == nil {
		t.Fatalf("[msg] allows empty message content")
	}

	err, _ = process(t, []rune("/msg lamer " + content))
	if err == nil {
		t.Fatalf("[msg] allows nonexistent recipients")
	}

	err, payload = process(t, []rune("/msg f00b47 " + content))
	if err != nil {
		t.Fatalf("[msg] valid recipient by address fails")
	}
	if content != payload {
		t.Fatalf("[msg] returned message content should be '%s', got '%s'", content, payload)
	}

	err, _ = process(t, []rune("/msg bar " + content))
	if err != nil {
		t.Fatalf("[msg] valid recipient by nick fails")
	}

	// del

	err, _ = process(t, []rune("/del lamer"))
	if err == nil {
		t.Fatalf("[del] allows invalid handle")
	}

	err, _ = process(t, []rune("/del foo"))
	if err != nil {
		t.Fatalf("[del] valid handle fails")
	}
	i = len(client.Sources)
	if i != 1 {
		t.Fatalf("should have 1 source, have %d", i)
	}

	err, _ = process(t, []rune("/add foo deadbeef"))
	if err != nil {
		t.Fatalf("[add/del] re-add of 'foo deadbeef' after delete fails")
	}
	i = len(client.Sources)
	if i != 2 {
		t.Fatalf("should have 2 source, have %d", i)
	}

	process(t, []rune("/del foo"))
	err, _ = process(t, []rune("/add foo beeffeed"))
	if err != nil {
		t.Fatalf("[add/del] add of new address for foo 'beeffeed' after delete fails")
	}
	if i != 2 {
		t.Fatalf("should have 2 source, have %d", i)
	}
	checkbytes["foo"] = []byte{0xbe, 0xef, 0xfe, 0xed}

	if client.Sources["foo"] == nil {
		t.Fatalf("source handle 'foo' doesnt exist")
	}
	if client.Sources["bar"] == nil {
		t.Fatalf("source handle 'bar' doesnt exist")
	}
	if client.Sources["foo"].Nick != "foo" {
		t.Fatalf("source handle 'foo' should have nick 'foo', has '%s'", client.Sources["foo"].Nick)
	}
	if client.Sources["bar"].Nick != "bar" {
		t.Fatalf("source handle 'bar' should have nick 'bar', has '%s'", client.Sources["bar"].Nick)
	}

	if !bytes.Equal(client.Sources["foo"].Addr, checkbytes["foo"])  {
		t.Fatalf("source addr for handle 'foo' should be '%x', is '%x'", checkbytes["foo"], client.Sources["foo"].Addr)
	}
	if !bytes.Equal(client.Sources["bar"].Addr, checkbytes["bar"])  {
		t.Fatalf("source addr for handle 'bar' should be '%x', is '%x'", checkbytes["bar"], client.Sources["bar"].Addr)
	}

	// dump the sources if we are verbose
	if debugflag {
		for key, src := range client.Sources {
			t.Logf("#%d: (%s) %s: %x", i, key, src.Nick, src.Addr)
		}
	}
}

func process(t *testing.T, line []rune) (error, string) {
	c, p, e := client.Process(line)
	if debugflag {
		t.Logf("\n'%s'\n > cmd:\t%s\n > payload:\t%s\n > err:\t%v\n\n", string(line), c, p, e)
	}
	return e, p
}
