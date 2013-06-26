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
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/uwedeportivo/torrentzip/czip"
)

const (
	fileHeaderSignature      = 0x04034b50
	directoryHeaderSignature = 0x02014b50
	directoryEndSignature    = 0x06054b50
	fileHeaderLen            = 30 // + filename + extra
	directoryHeaderLen       = 46 // + filename + extra + comment
	directoryEndLen          = 22 // + comment

	creatorFAT = 0

	zipVersion20 = 20 // 2.0
)

type Writer struct {
	uw   *czip.Writer
	sink io.Writer
	tf   *os.File
	bf   *bufio.Writer
}

func NewWriter(w io.Writer) (*Writer, error) {
	r := new(Writer)

	tf, err := ioutil.TempFile("", "torrentzip")
	if err != nil {
		return nil, err
	}
	r.tf = tf
	r.bf = bufio.NewWriter(r.tf)

	r.sink = w
	r.uw = czip.NewWriter(r.bf)
	return r, nil
}

type fileIndex struct {
	name   string
	index  int
	offset int64
}

type fileIndices []*fileIndex

func (fis fileIndices) Len() int {
	return len(fis)
}

func (fis fileIndices) Swap(i, j int) {
	fis[i], fis[j] = fis[j], fis[i]
}

func (fis fileIndices) Less(i, j int) bool {
	return strings.ToLower(fis[i].name) < strings.ToLower(fis[j].name)
}

func (w *Writer) Close() error {
	err := w.uw.Close()
	if err != nil {
		return err
	}

	err = w.bf.Flush()
	if err != nil {
		return err
	}

	err = w.tf.Close()
	if err != nil {
		return err
	}

	r, err := czip.OpenReader(w.tf.Name())
	if err != nil {
		return err
	}

	fis := make(fileIndices, len(r.File))

	for k, zf := range r.File {
		fis[k] = &fileIndex{
			name:  zf.Name,
			index: k,
		}
	}

	sort.Sort(fis)

	cw := &countWriter{
		w: w.sink,
	}

	for _, fi := range fis {
		fh := r.File[fi.index]
		fi.offset = cw.count
		err = writeHeader(cw, fh)
		if err != nil {
			return err
		}
		fr, err := fh.OpenRaw()
		if err != nil {
			return err
		}

		_, err = io.Copy(cw, fr)
		if err != nil {
			return err
		}
	}

	dircrc := crc32.NewIEEE()
	mw := io.MultiWriter(cw, dircrc)
	start := cw.count

	for _, fi := range fis {
		fh := r.File[fi.index]
		err = writeCentralHeader(mw, fh, fi.offset)
		if err != nil {
			return err
		}
	}
	end := cw.count

	var buf [directoryEndLen]byte
	b := writeBuf(buf[:])
	b.uint32(uint32(directoryEndSignature))
	b.uint16(0)
	b.uint16(0)
	b.uint16(uint16(len(fis)))
	b.uint16(uint16(len(fis)))
	b.uint32(uint32(end - start))
	b.uint32(uint32(start))
	b.uint16(22)

	zipcomment := "TORRENTZIPPED-" + strings.ToUpper(hex.EncodeToString(dircrc.Sum(nil)))

	if _, err := cw.Write(buf[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(cw, zipcomment); err != nil {
		return err
	}

	if err := r.Close(); err != nil {
		return err
	}

	if err := os.Remove(w.tf.Name()); err != nil {
		return err
	}

	return nil
}

func writeHeader(w io.Writer, h *czip.File) error {
	var buf [fileHeaderLen]byte
	b := writeBuf(buf[:])
	b.uint32(uint32(fileHeaderSignature))
	b.uint16(zipVersion20)
	b.uint16(2)
	b.uint16(8)
	b.uint16(48128)
	b.uint16(8600)
	b.uint32(h.CRC32)
	b.uint32(h.CompressedSize)
	b.uint32(h.UncompressedSize)
	b.uint16(uint16(len(h.Name)))
	b.uint16(0)
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(w, h.Name); err != nil {
		return err
	}
	return nil
}

func writeCentralHeader(w io.Writer, h *czip.File, offset int64) error {
	var buf [directoryHeaderLen]byte
	b := writeBuf(buf[:])
	b.uint32(uint32(directoryHeaderSignature))
	b.uint16(creatorFAT)
	b.uint16(zipVersion20)
	b.uint16(2)
	b.uint16(8)
	b.uint16(48128)
	b.uint16(8600)
	b.uint32(h.CRC32)
	b.uint32(h.CompressedSize)
	b.uint32(h.UncompressedSize)
	b.uint16(uint16(len(h.Name)))
	b.uint16(0)
	b.uint16(0)
	b.uint16(0)
	b.uint16(0)
	b.uint32(0)
	b.uint32(uint32(offset))
	if _, err := w.Write(buf[:]); err != nil {
		return err
	}
	if _, err := io.WriteString(w, h.Name); err != nil {
		return err
	}
	return nil
}

func (w *Writer) Create(name string) (io.Writer, error) {
	return w.uw.Create(name)
}

type writeBuf []byte

func (b *writeBuf) uint16(v uint16) {
	binary.LittleEndian.PutUint16(*b, v)
	*b = (*b)[2:]
}

func (b *writeBuf) uint32(v uint32) {
	binary.LittleEndian.PutUint32(*b, v)
	*b = (*b)[4:]
}

func (b *writeBuf) uint64(v uint64) {
	binary.LittleEndian.PutUint64(*b, v)
	*b = (*b)[8:]
}

type countWriter struct {
	w     io.Writer
	count int64
}

func (w *countWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.count += int64(n)
	return n, err
}
