package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/uwedeportivo/torrentzip/czip"
)

func main() {
	outpath := flag.String("out", "", "zip file")

	flag.Parse()

	file, err := os.Create(*outpath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating zip file %s failed: %v\n", *outpath, err)
		os.Exit(1)
	}

	zw := czip.NewWriter(file)

	for _, name := range flag.Args() {
		if filepath.IsAbs(name) {
			fmt.Fprintf(os.Stderr, "cannot add absolute paths to a zip file:  %s\n", name)
			os.Exit(1)
		}

		cf, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "opening file %s failed: %v\n", name, err)
			os.Exit(1)
		}
		fh, err := zw.Create(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "creating zip entry for file %s failed: %v\n", name, err)
			os.Exit(1)
		}

		_, err = io.Copy(fh, cf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "copying to zip entry for file %s failed: %v\n", name, err)
			os.Exit(1)
		}
		cf.Close()
	}
	zw.Close()
	file.Close()
}
