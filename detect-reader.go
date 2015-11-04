package kaito

import "io"

type Codec int

const (
	CODEC_UNKNOWN = iota
	CODEC_GZIP    = iota
	CODEC_BZIP2   = iota
	CODEC_XZ      = iota
)

type CodecDetectReader struct {
	in          io.Reader
	header      []byte
	codec       Codec
	alreadyRead bool
}

func NewCodecDetectReader(r io.Reader) *CodecDetectReader {
	return &CodecDetectReader{in: r}
}

func (cdr *CodecDetectReader) Detect() (Codec, error) {
	if cdr.header == nil {
		buf := make([]byte, 6) // We need at most 6 bytes to detect xz format.
		n, err := cdr.in.Read(buf)
		if err != nil {
			return CODEC_UNKNOWN, err
		}
		cdr.header = buf[0:n]
		switch {
		case n >= 2 && buf[0] == 0x1F && buf[1] == 0x8B: // Gzip
			cdr.codec = CODEC_GZIP
		case n >= 3 && buf[0] == 'B' && buf[1] == 'Z' && buf[2] == 'h': // Bzip2
			cdr.codec = CODEC_BZIP2
		case n >= 6 && buf[0] == 0xFD && buf[1] == '7' && buf[2] == 'z' && buf[3] == 'X' && buf[4] == 'Z' && buf[5] == 0x00: // Xz
			cdr.codec = CODEC_XZ
		}
	}
	return cdr.codec, nil
}

func (cdr *CodecDetectReader) Read(p []byte) (n int, err error) {
	_, err = cdr.Detect() // Read header if it is not read yet.
	if err != nil {
		return 0, err
	}
	if len(cdr.header) > 0 { // header is not read yet
		if len(p) < len(cdr.header) {
			copy(p, cdr.header[0:len(p)])
			cdr.header = cdr.header[len(p):]
			return len(p), nil
		} else {
			copy(p[0:len(cdr.header)], cdr.header)
			n = len(cdr.header)
			m, err := cdr.in.Read(p[n:])
			cdr.header = []byte{}
			return n + m, err
		}
	} else {
		return cdr.in.Read(p)
	}
}
