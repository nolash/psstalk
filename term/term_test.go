package term

import (
	"testing"
)

func Test_Process(t *testing.T) {
	var c string
	var b string
	var err error
	var line []rune

	client := NewTalkClient(1)

	line = []rune("/foo bar")
	c, b, err = client.Process(line)
	t.Logf("cmd %q, bytes %x, err: %v", c, b, err)


	line = []rune("/add foo")
	c, b, err = client.Process(line)
	t.Logf("cmd %q, bytes %x, err: %v", c, b, err)

	line = []rune("/add foo deadbeef")
	c, b, err = client.Process(line)
	t.Logf("cmd %q, bytes %x, err: %v", c, b, err)

	line = []rune("/add foo deadbeef")
	c, b, err = client.Process(line)
	t.Logf("cmd %q, bytes %x, err: %v", c, b, err)

	line = []rune("/add bar f00b4742 psst psst")
	c, b, err = client.Process(line)
	t.Logf("cmd %q, bytes %x, err: %v", c, b, err)

	i := 0
	for key, src := range client.Sources {
		i++
		t.Logf("#%d: (%s) %s: %x", i, key, src.Nick, src.Addr)
	}
}
