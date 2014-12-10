package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/uwedeportivo/torrentzip/czip"
)

func main() {
	flag.Parse()

	if len(flag.Args()) == 0 {
		os.Exit(1)
	}

	name := flag.Args()[0]

	zr, err := czip.OpenReader(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "opening zip file %s failed: %v\n", name, err)
		os.Exit(1)
	}

	for _, f := range zr.File {
		fmt.Printf("Size of %s: %d \n", f.Name, f.UncompressedSize64)

		dwf, err := os.Create("z-" + f.Name)
		if err != nil {
			log.Fatal(err)
		}

		rc, err := f.Open()
		if err != nil {
			log.Fatal(err)
		}
		_, err = io.Copy(dwf, rc)
		if err != nil {
			log.Fatal(err)
		}
		rc.Close()
		dwf.Close()
	}

	zr.Close()
}
