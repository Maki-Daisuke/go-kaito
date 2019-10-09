package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/Maki-Daisuke/go-kaito"
	"github.com/jessevdk/go-flags"
)

var opts struct {
	DisableGzip  bool `short:"G" long:"disable-gzip" description:"Disable Gzip decompression and pass through raw input."`
	DisableBzip2 bool `short:"B" long:"disable-bzip2" description:"Disable Bzip2 decompression and pass through raw input."`
	DisableXz    bool `short:"X" long:"disable-xz" description:"Disable Xz decompression and pass through raw input."`
	ForceNative  bool `short:"n" long:"force-native" description:"Force to use Go-native implentation of decompression algorithm."`
	ToStdout     bool `short:"c" long:"stdout" description:"Write the decompressed data to standard output instead of a file. This implies --keep."`
	Keep         bool `short:"k" long:"keep" description:"Don't delete the input files."`
	Decode       bool `short:"d" long:"decompress" description:"Nop. Just for tar command."`
}

func main() {
	args, err := flags.Parse(&opts)
	if err != nil {
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
		if file == "-" {
			k := kaito.NewWithOptions(os.Stdin, kaitoOpts)
			_, err := io.Copy(os.Stdout, k)
			if err != nil {
				fmt.Fprintf(os.Stderr, "kaito: %s\n", err)
			}
		} else {
			var err error
			rd, err := os.Open(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "kaito: %s\n", err)
				continue
			}
			out := os.Stdout
			if !opts.ToStdout {
				outFile, err := outputFilename(file)
				if err != nil {
					fmt.Fprintf(os.Stderr, "kaito: %s: Filename has an unknown suffix, skipping\n", file)
					continue
				}
				out, err = os.OpenFile(outFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
				if err != nil {
					fmt.Fprintf(os.Stderr, "kaito: %s, skipping\n", err)
					continue
				}
			}
			k := kaito.NewWithOptions(rd, kaitoOpts)
			_, err = io.Copy(out, k)
			if err != nil {
				fmt.Fprintf(os.Stderr, "kaito: %s: %s\n", file, err)
			}
			if !opts.Keep {
				err := os.Remove(file)
				if err != nil {
					fmt.Fprintf(os.Stderr, "kaito: %s: %s\n", file, err)
				}
			}
		}
	}
}

var reExt *regexp.Regexp = regexp.MustCompile(`(?i)\.(?:gz|bz2|xz)$`)

func outputFilename(s string) (string, error) {
	t := reExt.FindString(s)
	if t == "" {
		return "", errors.New("unknown suffix")
	}
	return s[0 : len(s)-len(t)], nil
}
