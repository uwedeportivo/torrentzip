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
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/cheggaaa/pb"
	"github.com/uwedeportivo/torrentzip"
	"github.com/uwedeportivo/torrentzip/czip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	versionStr = "1.0"
	megabyte   = 1 << 20
)

func usage() {
	fmt.Fprintf(os.Stderr, "%s version %s, Copyright (c) 2013 Uwe Hoffmann. All rights reserved.\n", os.Args[0], versionStr)
	fmt.Fprintf(os.Stderr, "\tUsage: %s -faildir <faildir> <dir 1> ..... <dir n>\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nFlag defaults:\n")
	flag.PrintDefaults()
}

type workUnit struct {
	path string
	size int64
}

type scanVisitor struct {
	inwork chan *workUnit
}

func (sv *scanVisitor) visit(path string, f os.FileInfo, err error) error {
	if !f.IsDir() && filepath.Ext(path) == ".zip" {
		sv.inwork <- &workUnit{
			path: path,
			size: f.Size(),
		}
	}
	return nil
}

type testWorker struct {
	failpath     string
	byteProgress *pb.ProgressBar
	inwork       chan *workUnit
	wg           *sync.WaitGroup
}

func (tw *testWorker) run() {
	for wu := range tw.inwork {
		path := wu.path
		goldensha1, err := hashZip(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot read from %s: %v, copying it into %s\n", path, err, tw.failpath)
			_, err = tw.copyFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to copy %s into %s: %v\n", path, tw.failpath, err)
			}
		}

		err = testZip(path, goldensha1)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zip test failed for %s failed with: %v, copying it into %s\n", path, err, tw.failpath)
			_, err = tw.copyFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to copy %s into %s: %v\n", path, tw.failpath, err)
			}
		}

		tw.byteProgress.Add(int(wu.size / megabyte))
		tw.wg.Done()
	}
}

func (tw *testWorker) copyFile(src string) (int64, error) {
	dst := filepath.Join(tw.failpath, filepath.Base(src))

	sf, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer sf.Close()
	df, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer df.Close()
	return io.Copy(df, sf)
}

func hashZip(path string) (string, error) {
	r, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	hh := sha1.New()

	_, err = io.Copy(hh, r)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hh.Sum(nil)), nil
}

func testZip(path string, goldensha1 string) error {
	r, err := czip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()

	w, err := ioutil.TempFile(os.TempDir(), "testtorrentzip")
	if err != nil {
		return err
	}

	hh := sha1.New()

	zw, err := torrentzip.NewWriter(io.MultiWriter(w, hh))
	if err != nil {
		return err
	}

	for _, fh := range r.File {
		cw, err := zw.Create(fh.Name)
		if err != nil {
			return err
		}
		cr, err := fh.Open()
		if err != nil {
			return err
		}

		_, err = io.Copy(cw, cr)
		if err != nil {
			return err
		}

		err = cr.Close()
		if err != nil {
			return err
		}
	}

	err = zw.Close()
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	err = os.Remove(w.Name())
	if err != nil {
		return err
	}

	testsha1 := hex.EncodeToString(hh.Sum(nil))

	if testsha1 != goldensha1 {
		return fmt.Errorf("produced torrentzip for %s differs from golden", path)
	}

	return nil
}

type countVisitor struct {
	numBytes int64
	numFiles int
}

func (cv *countVisitor) visit(path string, f os.FileInfo, err error) error {
	if !f.IsDir() && filepath.Ext(path) == ".zip" {
		cv.numFiles += 1
		cv.numBytes += f.Size()
	}
	return nil
}

func main() {
	flag.Usage = usage

	help := flag.Bool("help", false, "show this message")
	version := flag.Bool("version", false, "show version")

	failpath := flag.String("faildir", "", "dir where failed torrentzips should be copied")

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	if *version {
		fmt.Fprintf(os.Stdout, "%s version %s, Copyright (c) 2013 Uwe Hoffmann. All rights reserved.\n", os.Args[0], versionStr)
		os.Exit(0)
	}

	if *failpath == "" {
		flag.Usage()
		os.Exit(0)
	}

	cv := new(countVisitor)

	for _, name := range flag.Args() {
		err := filepath.Walk(name, cv.visit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to count in dir %s: %v\n", name, err)
			os.Exit(1)
		}
	}

	byteProgress := pb.New(int(cv.numBytes / megabyte))
	byteProgress.RefreshRate = 5 * time.Second
	byteProgress.ShowCounters = true
	byteProgress.Start()

	inwork := make(chan *workUnit)

	sv := &scanVisitor{
		inwork: inwork,
	}

	wg := new(sync.WaitGroup)
	wg.Add(cv.numFiles)

	for i := 0; i < 8; i++ {
		worker := &testWorker{
			byteProgress: byteProgress,
			failpath:     *failpath,
			inwork:       inwork,
			wg:           wg,
		}

		go worker.run()
	}

	for _, name := range flag.Args() {
		err := filepath.Walk(name, sv.visit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to scan dir %s: %v\n", name, err)
			os.Exit(1)
		}
	}

	wg.Wait()

	byteProgress.FinishPrint("Done scanning")
	close(inwork)
}
