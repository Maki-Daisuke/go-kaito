package kaito

import (
	"compress/bzip2"
	"compress/gzip"
	"errors"
	"io"
	"os/exec"
	"runtime"

	"github.com/ulikunitz/xz"
)

type KaitoReader struct {
	io.Reader // Just delegate all Read operation to this Reader
}

func New(r io.Reader) io.Reader {
	return NewWithOptions(r, 0)
}

func NewWithOptions(r io.Reader, o Options) io.Reader {
	this := new(KaitoReader)
	this.Reader = newCodecDetectReader(this, r, o)
	return this
}

// We need to read at most 6 bytes to detect gzip, bzip2 and xz format.
const MAX_HEADER_LENGTH = 6

type codec_t int

const (
	CODEC_UNDETERMINED = iota
	CODEC_GZIP
	CODEC_BZIP2
	CODEC_XZ
)

var errUnknownCodec = errors.New("Unknown codec")

type codecDetectReader struct {
	kaito *KaitoReader // Pointer to KaitoReader
	in    io.Reader
	opts  Options
	buf   [MAX_HEADER_LENGTH]byte
	len   int // number of bytes read in buf
}

func newCodecDetectReader(k *KaitoReader, r io.Reader, o Options) *codecDetectReader {
	return &codecDetectReader{kaito: k, in: r, opts: o}
}

func (cdr *codecDetectReader) Detect() (codec_t, error) {
	isEOF := false
	n, err := cdr.in.Read(cdr.buf[cdr.len:])
	if err != nil {
		if err == io.EOF {
			isEOF = true
		} else {
			return CODEC_UNDETERMINED, err
		}
	}
	cdr.len += n
	// ここから下は、ホントはDFAを書いたほうが効率がよい
	if cdr.len >= 1 && cdr.buf[0] != 0x1F && cdr.buf[0] != 'B' && cdr.buf[0] != 0xFD {
		return CODEC_UNDETERMINED, errUnknownCodec
	}
	if cdr.len >= 2 && cdr.buf[0] == 0x1F {
		if cdr.buf[1] == 0x8B && !cdr.opts.IsDisableGzip() {
			return CODEC_GZIP, nil
		}
		return CODEC_UNDETERMINED, errUnknownCodec
	}
	if cdr.len >= 3 && cdr.buf[0] == 'B' && !cdr.opts.IsDisableBzip2() {
		if cdr.buf[1] == 'Z' && cdr.buf[2] == 'h' {
			return CODEC_BZIP2, nil
		}
		return CODEC_UNDETERMINED, errUnknownCodec
	}
	if cdr.len >= 6 && cdr.buf[0] == 0xFD {
		if cdr.buf[1] == '7' && cdr.buf[2] == 'z' && cdr.buf[3] == 'X' && cdr.buf[4] == 'Z' && cdr.buf[5] == 0x00 && !cdr.opts.IsDisableXz() {
			return CODEC_XZ, nil
		}
		return CODEC_UNDETERMINED, errUnknownCodec
	}
	if isEOF || cdr.len >= MAX_HEADER_LENGTH {
		return CODEC_UNDETERMINED, errUnknownCodec
	}
	return CODEC_UNDETERMINED, nil
}

func (cdr *codecDetectReader) Read(p []byte) (int, error) {
	var codec codec_t
	var err error
	for {
		codec, err = cdr.Detect() // Read header if it is not read yet
		if codec != CODEC_UNDETERMINED {
			break
		}
		if err != nil {
			if err == errUnknownCodec {
				err = nil // not error
				break
			}
			return copy(p, cdr.buf[0:cdr.len]), err
		}
	}
	switch codec {
	case CODEC_GZIP:
		err = cdr.initGzip()
	case CODEC_BZIP2:
		err = cdr.initBzip2()
	case CODEC_XZ:
		err = cdr.initXz()
	default: // Here, codec == CODEC_UNDETERMINED, it is treated as plain text
		r, w := io.Pipe()
		go func() {
			w.Write(cdr.buf[0:cdr.len])
			io.Copy(w, cdr.in)
			w.Close()
		}()
		cdr.kaito.Reader = r
	}
	if err != nil {
		return copy(p, cdr.buf[0:cdr.len]), err
	}
	return cdr.kaito.Reader.Read(p)
}

func (cdr *codecDetectReader) initGzip() error {
	if !cdr.opts.IsForceNative() {
		err := cdr.initCmd("gzip", "-cd")
		if err == nil {
			return nil
		}
		// Fallback through Golang-native implementation.
	}
	r, w := io.Pipe()
	go func() {
		w.Write(cdr.buf[0:cdr.len])
		io.Copy(w, cdr.in)
		w.Close()
	}()
	unzipped, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	cdr.kaito.Reader = unzipped
	return nil
}

func (cdr *codecDetectReader) initBzip2() error {
	if !cdr.opts.IsForceNative() {
		err := cdr.initCmd("bzip2", "-cd")
		if err == nil {
			return nil
		}
		// Fallback through Golang-native implementation.
	}
	r, w := io.Pipe()
	go func() {
		w.Write(cdr.buf[0:cdr.len])
		io.Copy(w, cdr.in)
		w.Close()
	}()
	cdr.kaito.Reader = bzip2.NewReader(r)
	return nil
}

func (cdr *codecDetectReader) initXz() error {
	if !cdr.opts.IsForceNative() {
		err := cdr.initCmd("xz", "-cd")
		if err == nil {
			return nil
		}
		// Fallback through Golang-native implementation.
	}
	r, w := io.Pipe()
	go func() {
		w.Write(cdr.buf[0:cdr.len])
		io.Copy(w, cdr.in)
		w.Close()
	}()
	unxz, err := (xz.ReaderConfig{SingleStream: true}).NewReader(r)
	if err != nil {
		return nil
	}
	cdr.kaito.Reader = unxz
	return nil
}

func (cdr *codecDetectReader) initCmd(c string, args ...string) (err error) {
	cmd := exec.Command(c, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	go func() {
		stdin.Write(cdr.buf[0:cdr.len])
		io.Copy(stdin, cdr.in)
		stdin.Close()
	}()
	cdr.kaito.Reader = stdout
	runtime.SetFinalizer(cdr.kaito, func(o interface{}) {
		stdin.Close()
		stdout.Close()
		cmd.Wait()
	})
	return nil
}
