package datafiles

import (
	"crypto/sha256"
	"encoding/binary"
)

// A counted additive SHA-256 multiset fingerprint verifies driver roundtrips
// without sorting or retaining the complete target table in memory. The count
// retains duplicate multiplicity; NULL and empty have different encodings.
func addRowDigest(sum *[32]byte, row []*string) {
	h := sha256.New()
	var length [8]byte
	for _, v := range row {
		if v == nil {
			h.Write([]byte{0})
			continue
		}
		h.Write([]byte{1})
		binary.LittleEndian.PutUint64(length[:], uint64(len(*v)))
		h.Write(length[:])
		h.Write([]byte(*v))
	}
	digest := h.Sum(nil)
	carry := 0
	for i := range sum {
		n := int(sum[i]) + int(digest[i]) + carry
		sum[i] = byte(n)
		carry = n >> 8
	}
}
