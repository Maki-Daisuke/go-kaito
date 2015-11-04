package kaito

import (
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
)

const (
	state_UNINITIALIZED = iota
	state_READY         = iota
	state_CLOSED        = iota
	state_ERROR         = iota
)

type Options struct {
	DisableGzip       bool
	DisableBzip2      bool
	DisableXz         bool
	ForceNativeDecode bool
}

type Reader struct {
	input        io.Reader
	opts         Options
	state        int
	cmd          *exec.Cmd
	decompressor io.Reader
}

func New(r io.Reader) io.ReadCloser {
	return &Reader{input: r}
}

func NewWithOptions(r io.Reader, o Options) io.ReadCloser {
	return &Reader{
		input: r,
		opts:  o,
	}
}

func (r *Reader) Read(p []byte) (n int, err error) {
	switch r.state {
	case state_UNINITIALIZED:
		n, err = r.initialize(p)
		if err != nil {
			if err == io.EOF {
				err = r.Close()
			} else {
				r.state = state_ERROR
			}
		}
		return
	case state_READY:
		n, err = r.decompressor.Read(p)
		if err != nil {
			if err == io.EOF {
				err = r.Close()
			} else {
				r.state = state_ERROR
			}
		}
		return
	case state_CLOSED:
		return 0, io.EOF
	case state_ERROR:
		return 0, fmt.Errorf("kaito is already at unusual state")
	default:
		panic("Should not enter here!!!")
	}
}

func (r *Reader) initialize(buf []byte) (int, error) {
	if len(buf) < 6 {
		return 0, fmt.Errorf(`Sorry, kaito requires at least 6 bytes for buffer.`)
	}
	n, err := r.input.Read(buf[0:6])
	if err != nil {
		return n, err
	}
	switch {
	case !r.opts.DisableGzip && buf[0] == 0x1F && buf[1] == 0x8B: // Gzip
		err := r.initializeGzip(buf[0:n])
		if err != nil {
			return 0, err
		}
		return r.decompressor.Read(buf)
	case !r.opts.DisableXz && buf[0] == 0xFD && buf[1] == '7' && buf[2] == 'z' && buf[3] == 'X' && buf[4] == 'Z' && buf[5] == 0x00: // Xz
		err := r.initializeXz(buf[0:n])
		if err != nil {
			return 0, err
		}
		return r.decompressor.Read(buf)
	case !r.opts.DisableBzip2 && buf[0] == 'B' && buf[1] == 'Z' && buf[2] == 'h': // Bzip2
		// As Bz2 uses only Alphabet characters as its magic number, it might cause misdetection. Thus, Bz2 is disabled by default.
		err := r.initializeBzip2(buf[0:n])
		if err != nil {
			return 0, err
		}
		return r.decompressor.Read(buf)
	default: // No compression
		r.decompressor = r.input
		m, err := r.decompressor.Read(buf[n:])
		return n + m, err
	}
}

func (r *Reader) initializeGzip(header []byte) error {
	if !r.opts.ForceNativeDecode {
		err := r.initializeCmd(header, "gzip", "-cd")
		if err == nil {
			return nil
		}
		// Fallback through Golang-native implementation.
	}
	return r.initializeNative(header, func(r io.Reader) (io.Reader, error) {
		return gzip.NewReader(r)
	})
}

func (r *Reader) initializeXz(header []byte) error {
	if r.opts.ForceNativeDecode {
		return fmt.Errorf("Go does not have Xz library by default.")
	}
	return r.initializeCmd(header, "xz", "-cd")
}

func (r *Reader) initializeBzip2(header []byte) error {
	if !r.opts.ForceNativeDecode {
		err := r.initializeCmd(header, "bzip2", "-cd")
		if err == nil {
			return nil
		}
		// Fallback through Golang-native implementation.
	}
	return r.initializeNative(header, func(r io.Reader) (io.Reader, error) {
		return bzip2.NewReader(r), nil
	})
}

func (r *Reader) initializeCmd(header []byte, cmd string, args ...string) (err error) {
	r.cmd = exec.Command(cmd, args...)
	r.decompressor, err = r.cmd.StdoutPipe()
	if err != nil {
		r.state = state_ERROR
		return
	}
	dst, err := r.cmd.StdinPipe()
	if err != nil {
		r.state = state_ERROR
		return
	}
	err = r.cmd.Start()
	if err != nil {
		r.state = state_ERROR
		return
	}
	_, err = dst.Write(header)
	if err != nil {
		r.state = state_ERROR
		return
	}
	r.state = state_READY
	go func() {
		io.Copy(dst, r.input)
		dst.Close()
	}()
	return nil
}

func (r *Reader) initializeNative(header []byte, newer func(io.Reader) (io.Reader, error)) (err error) {
	r.cmd = nil
	rd, wr := io.Pipe()
	copy_header := make([]byte, len(header))
	copy(copy_header, header)
	go func() {
		wr.Write(copy_header)
		io.Copy(wr, r.input)
		wr.Close()
	}()
	r.decompressor, err = newer(rd)
	if err != nil {
		r.state = state_ERROR
		return
	}
	r.state = state_READY
	return nil
}

func (r *Reader) Close() error {
	if r.state == state_CLOSED {
		// Already closed.
		return nil
	}
	if c, ok := r.input.(io.Closer); ok {
		c.Close()
	}
	if r.cmd != nil {
		//s := r.decompressor.(io.Closer)
		//err1 := s.Close()
		err := r.cmd.Process.Signal(os.Interrupt)
		if err != nil {
			return err
		}
		err2 := r.cmd.Wait()
		if err2 != nil {
			return err2
		}
		r.cmd = nil
	}
	r.input = nil
	r.decompressor = nil
	r.state = state_CLOSED
	return nil
}
