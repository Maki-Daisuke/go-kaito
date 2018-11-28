package kaito

import (
	"bufio"
	"io"
	"os"
	"testing"
)

func TestGzip(t *testing.T) {
	testOneToTen(t, "test/one-ten.txt.gz")
}

func TestXz(t *testing.T) {
	testOneToTen(t, "test/one-ten.txt.gz")
}

func TestBzip2Disabled(t *testing.T) {
	file, err := os.Open("test/one-ten.txt.bz2")
	if err != nil {
		t.Fatal(err)
	}
	rd := NewWithOptions(file, DisableBzip2)
	buf := make([]byte, 6)
	n, err := rd.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if n < 3 {
		t.Fatalf("Expected at least 3 bytes, but actually only %d bytes are read", n)
	}
	if buf[0] != 'B' || buf[1] != 'Z' || buf[2] != 'h' {
		t.Fatalf(`Expected "BZh" as magic number of Bzip2, but actually got %s`, string(buf))
	}
}

func TestBzip2(t *testing.T) {
	testOneToTen(t, "test/one-ten.txt.bz2")
}

func TestGzipNative(t *testing.T) {
	testOneToTenWithOpts(t, "test/one-ten.txt.gz", ForceNative)
}

func TestXzNative(t *testing.T) {
	// Golang does not have Xz decompressor in its standard library. So, this will fail.
	file, err := os.Open("test/one-ten.txt.xz")
	if err != nil {
		t.Fatal(err)
	}
	rd := NewWithOptions(file, ForceNative)
	brd := bufio.NewReader(rd)
	_, err = brd.ReadString('\n')
	if err == nil {
		t.Fatal("Read() should fail, but actually successed. Whoa!?")
	}
}

func TestBzip2Native(t *testing.T) {
	testOneToTenWithOpts(t, "test/one-ten.txt.bz2", ForceNative)
}

func testOneToTen(t *testing.T, name string) {
	file, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	rd := New(file)
	testOneToTen_aux(t, rd)
}

func testOneToTenWithOpts(t *testing.T, name string, opts Options) {
	file, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	rd := NewWithOptions(file, opts)
	testOneToTen_aux(t, rd)
}

func testOneToTen_aux(t *testing.T, rd io.Reader) {
	brd := bufio.NewReader(rd)
	for _, num := range []string{"One\n", "Two\n", "Three\n", "Four\n", "Five\n", "Six\n", "Seven\n", "Eight\n", "Nine\n", "Ten\n"} {
		line, err := brd.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if line != num {
			t.Fatalf("Expected %v, but actually %v", num, line)
		}
	}
	_, err := brd.ReadString('\n')
	if err != io.EOF {
		t.Fatalf("Expected EOF, but actually %v", err)
	}
}
