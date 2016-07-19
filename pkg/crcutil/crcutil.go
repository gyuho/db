package crcutil

import (
	"hash"
	"hash/crc32"
)

// Size of a CRC-32 checksum in bytes.
const Size = 4

type digest struct {
	crc uint32
	tab *crc32.Table
}

// New creates a new hash.Hash32 computing the CRC-32 checksum
// using the polynomial represented by the Table.
func New(prev uint32, tab *crc32.Table) hash.Hash32 {
	return &digest{
		crc: prev,
		tab: tab,
	}
}

func (d *digest) Size() int {
	return Size
}

func (d *digest) BlockSize() int {
	return 1
}

func (d *digest) Reset() {
	d.crc = 0
}

func (d *digest) Write(p []byte) (int, error) {
	d.crc = crc32.Update(d.crc, d.tab, p)
	return len(p), nil
}

func (d *digest) Sum32() uint32 {
	return d.crc
}

func (d *digest) Sum(in []byte) []byte {
	s := d.Sum32()
	return append(in, byte(s>>24), byte(s>>16), byte(s>>8), byte(s))
}
