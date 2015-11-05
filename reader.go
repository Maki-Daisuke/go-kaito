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
	state_READY
	state_CLOSED
	state_ERROR
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
		err = r.initialize()
		if err != nil {
			return 0, err
		}
		n, err = r.decompressor.Read(p)
		if err != nil {
			if err == io.EOF {
				r.Close()
			} else {
				r.state = state_ERROR
			}
		}
		return
	case state_READY:
		n, err = r.decompressor.Read(p)
		if err != nil {
			if err == io.EOF {
				r.Close()
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

func (r *Reader) initialize() error {
	cdr := NewCodecDetectReader(r.input)
	codec, err := cdr.Detect()
	if err != nil {
		return err
	}
	switch {
	case !r.opts.DisableGzip && codec == CODEC_GZIP:
		err := r.initializeGzip(cdr)
		if err != nil {
			return err
		}
	case !r.opts.DisableBzip2 && codec == CODEC_BZIP2:
		err := r.initializeBzip2(cdr)
		if err != nil {
			return err
		}
	case !r.opts.DisableXz && codec == CODEC_XZ:
		err := r.initializeXz(cdr)
		if err != nil {
			return err
		}
	default: // No compression
		r.decompressor = cdr
	}
	return nil
}

func (r *Reader) initializeGzip(in io.Reader) error {
	if !r.opts.ForceNativeDecode {
		err := r.initializeCmd(in, "gzip", "-cd")
		if err == nil {
			return nil
		}
		// Fallback through Golang-native implementation.
	}
	r.cmd = nil
	unzipper, err := gzip.NewReader(in)
	if err != nil {
		return err
	}
	r.decompressor = unzipper
	r.state = state_READY
	return nil
}

func (r *Reader) initializeBzip2(in io.Reader) error {
	if !r.opts.ForceNativeDecode {
		err := r.initializeCmd(in, "bzip2", "-cd")
		if err == nil {
			return nil
		}
		// Fallback through Golang-native implementation.
	}
	r.cmd = nil
	r.decompressor = bzip2.NewReader(in)
	r.state = state_READY
	return nil
}

func (r *Reader) initializeXz(in io.Reader) error {
	if r.opts.ForceNativeDecode {
		return fmt.Errorf("Go does not have Xz library by default.")
	}
	return r.initializeCmd(in, "xz", "-cd")
}

func (r *Reader) initializeCmd(in io.Reader, cmd string, args ...string) (err error) {
	r.cmd = exec.Command(cmd, args...)
	r.cmd.Stdin = in
	r.decompressor, err = r.cmd.StdoutPipe()
	if err != nil {
		r.state = state_ERROR
		return
	}
	err = r.cmd.Start()
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
		err := r.cmd.Process.Signal(os.Interrupt)
		if err != nil {
			return err
		}
		r.cmd.Wait() // Ignore exit code.
		r.cmd = nil
	}
	r.input = nil
	r.decompressor = nil
	r.state = state_CLOSED
	return nil
}
