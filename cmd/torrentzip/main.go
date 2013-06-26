// Copyright (c) 2013 Uwe Hoffmann. All rights reserved.

/*
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/cheggaaa/pb"
	"github.com/uwedeportivo/torrentzip"
	"io"
	"os"
	"time"
)

const (
	versionStr = "1.0"
)

func usage() {
	fmt.Fprintf(os.Stderr, "%s version %s, Copyright (c) 2013 Uwe Hoffmann. All rights reserved.\n", os.Args[0], versionStr)
	fmt.Fprintf(os.Stderr, "\tUsage: %s -out <zipfile> <file 1> <file 2> ..... <file n>\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nFlag defaults:\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage

	help := flag.Bool("help", false, "show this message")
	version := flag.Bool("version", false, "show version")

	outpath := flag.String("out", "", "zip file")

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *version {
		fmt.Fprintf(os.Stdout, "%s version %s, Copyright (c) 2013 Uwe Hoffmann. All rights reserved.\n", os.Args[0], versionStr)
		os.Exit(0)
	}

	if *outpath == "" {
		flag.Usage()
		os.Exit(0)
	}

	file, err := os.Create(*outpath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating zip file %s failed: %v\n", *outpath, err)
		os.Exit(1)
	}
	defer file.Close()

	bf := bufio.NewWriter(file)
	defer bf.Flush()

	hh := sha1.New()

	zw, err := torrentzip.NewWriter(io.MultiWriter(bf, hh))
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating zip file writer %s failed: %v\n", *outpath, err)
		os.Exit(1)
	}

	progress := pb.New(len(flag.Args()) + 1)
	progress.RefreshRate = 5 * time.Second
	progress.ShowCounters = false
	progress.Start()

	for _, name := range flag.Args() {
		cf, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "opening file %s failed: %v\n", name, err)
			os.Exit(1)
		}
		defer cf.Close()

		fh, err := zw.Create(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot create zip header for file %s: %v\n", name, err)
			os.Exit(1)
		}

		_, err = io.Copy(fh, cf)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write file %s into zip: %v\n", name, err)
			os.Exit(1)
		}

		progress.Increment()
	}

	err = zw.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to close zip file %s: %v\n", *outpath, err)
		os.Exit(1)
	}

	progress.Increment()

	progress.Finish()

	fmt.Fprintf(os.Stdout, "finished creating zip file: %s\n", *outpath)
	fmt.Fprintf(os.Stdout, "sha1 of created zip file: %s\n", hex.EncodeToString(hh.Sum(nil)))
}
