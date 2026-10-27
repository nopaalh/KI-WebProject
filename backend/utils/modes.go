package utils

import "encoding/binary"

// CTRXor meng-XOR data dengan keystream mode CTR dan mengembalikan slice baru.
// Blok counter ke-i berisi (start + i) sebagai angka big-endian di 8 byte terakhir dari blockSize byte.
// encrypt adalah enkripsi satu blok (blockSize byte masuk, blockSize byte keluar) dan tidak boleh
// mengubah inputnya. Tanpa padding: blok terakhir memakai sebagian keystream, jadi hasilnya sepanjang data.
// Enkripsi dan dekripsi memakai fungsi yang sama. blockSize minimal 8.
func CTRXor(encrypt func([]byte) []byte, blockSize int, start uint64, data []byte) []byte {
	output := make([]byte, len(data))
	counterBlock := make([]byte, blockSize)
	for offset := 0; offset < len(data); offset += blockSize {
		binary.BigEndian.PutUint64(counterBlock[blockSize-8:], start+uint64(offset/blockSize))
		keystream := encrypt(counterBlock)
		for i := 0; i < blockSize && offset+i < len(data); i++ {
			output[offset+i] = data[offset+i] ^ keystream[i]
		}
	}
	return output
}
