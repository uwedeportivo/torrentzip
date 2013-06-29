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

package torrentzip

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/uwedeportivo/torrentzip/czip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	zipext = ".zip"
)

type testdataVisitor struct {
	t *testing.T
}

func (tv *testdataVisitor) visit(path string, f os.FileInfo, err error) error {
	if filepath.Ext(path) == zipext {
		r, err := czip.OpenReader(path)
		if err != nil {
			return err
		}
		defer r.Close()

		w, err := ioutil.TempFile(os.TempDir(), "torrentzip_test")
		if err != nil {
			return err
		}

		hh := sha1.New()

		zw, err := NewWriter(io.MultiWriter(w, hh))
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

			io.Copy(cw, cr)

			cr.Close()
		}

		err = zw.Close()
		if err != nil {
			return err
		}

		err = w.Close()
		if err != nil {
			return err
		}

		testsha1 := hex.EncodeToString(hh.Sum(nil))
		goldensha1 := strings.TrimSuffix(filepath.Base(path), zipext)

		if testsha1 != goldensha1 {
			return fmt.Errorf("produced torrentzip for %s differs from golden", path)
		}

		err = os.Remove(w.Name())
		if err != nil {
			return err
		}
	}
	return nil
}

func executeTest(t *testing.T, dir string) {
	tv := &testdataVisitor{
		t: t,
	}

	err := filepath.Walk(dir, tv.visit)
	if err != nil {
		t.Fatalf("test failed %v", err)
	}
}

func TestTorrrent(t *testing.T) {
	executeTest(t, "testdata")
}

func TestBigTorrrent(t *testing.T) {
	if testing.Short() {
		t.Skip("slow test; skipping")
	}
	executeTest(t, "bigtestdata")
}
