package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Maki-Daisuke/go-kaito"
	"github.com/jessevdk/go-flags"
)

var opts struct {
	DisableGzip  bool `short:"G" long:"disable-gzip" description:"Disable Gzip decompression and pass through raw input."`
	DisableBzip2 bool `short:"B" long:"disable-bzip2" description:"Disable Bzip2 decompression and pass through raw input."`
	DisableXz    bool `short:"X" long:"disable-xz" description:"Disable Xz decompression and pass through raw input."`
	ForceNative  bool `short:"n" long:"force-native" description:"Force to use Go-native implentation of decompression algorithm (this makes xz decompression fail)."`
	ToStdout     bool `short:"c" long:"stdout" description:"Write the decompressed data to standard output instead of a file. This implies --keep."`
	Keep         bool `short:"k" long:"keep" description:"Don't delete the input files."`
	Decode       bool `short:"d" long:"decompress" description:"Nop. Just for tar command."`
}

func main() {
	args, err := flags.Parse(&opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if opts.ToStdout {
		opts.Keep = true
	}

	var kaitoOpts kaito.Options
	if opts.DisableGzip {
		kaitoOpts |= kaito.DisableGzip
	}
	if opts.DisableBzip2 {
		kaitoOpts |= kaito.DisableBzip2
	}
	if opts.DisableXz {
		kaitoOpts |= kaito.DisableXz
	}
	if opts.ForceNative {
		kaitoOpts |= kaito.ForceNative
	}

	if len(args) == 0 { // Filter mode
		args = append(args, "-")
	}

	for _, file := range args {
		var r io.Reader
		if file == "-" {
			r = os.Stdin
		} else {
			var err error
			r, err = os.Open(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Can't open file %s: %s", file, err)
				continue
			}
		}
		k := kaito.NewWithOptions(r, kaitoOpts)
		_, err := io.Copy(os.Stdout, k)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
}
