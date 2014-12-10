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
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cheggaaa/pb"
)

const (
	versionStr = "1.0"
)

func usage() {
	fmt.Fprintf(os.Stderr, "%s version %s, Copyright (c) 2013 Uwe Hoffmann. All rights reserved.\n", os.Args[0], versionStr)
	fmt.Fprintf(os.Stderr, "\tUsage: %s -out <zipfile> <file or dir 1> <file or dir 2> ..... <file or dir n>\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nFlag defaults:\n")
	flag.PrintDefaults()
}

type adderVisitor struct {
	zw             *zip.Writer
	pwdName        string
	skipFirstLevel bool
}

func (av *adderVisitor) relativeName(path string) (string, error) {
	rv := path
	if filepath.IsAbs(rv) {
		rel, err := filepath.Rel(av.pwdName, rv)
		if err != nil {
			return "", err
		}
		rv = rel
	}

	if av.skipFirstLevel {
		si := strings.Index(rv, string(filepath.Separator))
		if si != -1 && len(rv) > si {
			rv = rv[si+1:]
		}
	}
	return rv, nil
}

func (av *adderVisitor) visit(path string, f os.FileInfo, err error) error {
	if f == nil {
		return fmt.Errorf("received nil os.FileInfo while visiting %s", path)
	}
	if f.IsDir() {
		isEmpty, err := dirEmpty(path)
		if err != nil {
			return err
		}

		if isEmpty {
			var buf bytes.Buffer

			relname, err := av.relativeName(path)
			if err != nil {
				return err
			}

			fh, err := av.zw.Create(relname + "/")
			if err != nil {
				return err
			}

			_, err = io.Copy(fh, &buf)
			if err != nil {
				return err
			}
		}
	} else {
		cf, err := os.Open(path)
		if err != nil {
			return err
		}
		defer cf.Close()

		relname, err := av.relativeName(path)
		if err != nil {
			return err
		}

		fh, err := av.zw.Create(relname)
		if err != nil {
			return err
		}

		_, err = io.Copy(fh, cf)
		if err != nil {
			return err
		}
	}
	return nil
}

func dirEmpty(dirname string) (bool, error) {
	fs, err := ioutil.ReadDir(dirname)
	if err != nil {
		return false, err
	}
	return len(fs) == 0, nil
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

	zw := zip.NewWriter(io.MultiWriter(bf, hh))

	progress := pb.New(len(flag.Args()) + 1)
	progress.RefreshRate = 5 * time.Second
	progress.ShowCounters = false
	progress.Start()

	pwdName, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot establish current working directory %v\n", err)
		os.Exit(1)
	}

	av := &adderVisitor{
		zw:      zw,
		pwdName: pwdName,
	}

	for _, name := range flag.Args() {
		if filepath.IsAbs(name) {
			fmt.Fprintf(os.Stderr, "cannot add absolute paths to a zip file:  %s\n", name)
			os.Exit(1)
		}

		av.skipFirstLevel = strings.HasSuffix(name, string(filepath.Separator))

		err = filepath.Walk(name, av.visit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "adding files from %s failed: %v\n", name, err)
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
