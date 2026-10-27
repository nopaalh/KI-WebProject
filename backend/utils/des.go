package utils

// utility function
// DESPermutation
func DESPermutation(input []byte, table []byte) []byte {
	output := make([]byte, len(table))
	for i := 0; i < len(table); i++ {
		output[i] = input[table[i]-1]
	}
	return output
}

// DESSplit membelah 64 bit menjadi l0 (32 bit kiri) dan r0 (32 bit kanan).
func DESSplit(plain []byte) (l0, r0 []byte) {
	l0 = plain[:32]
	r0 = plain[32:]

	return l0, r0
}

// DESRotateLeft memutar bits ke kiri sebanyak shift posisi, bit paling kiri pindah ke paling kanan.
// Hasilnya slice baru, bits tidak diubah.
func DESRotateLeft(bits []byte, shift int) []byte {
	temp := make([]byte, len(bits))
	for i := 0; i < len(bits); i++ {
		temp[i] = bits[(i+shift)%len(bits)]
	}
	return temp
}

// Plaintext sampe sebelum feistel round
// buat pengacak pertama plaintext
var desIP = [64]byte{
	58, 50, 42, 34, 26, 18, 10, 2,
	60, 52, 44, 36, 28, 20, 12, 4,
	62, 54, 46, 38, 30, 22, 14, 6,
	64, 56, 48, 40, 32, 24, 16, 8,
	57, 49, 41, 33, 25, 17, 9, 1,
	59, 51, 43, 35, 27, 19, 11, 3,
	61, 53, 45, 37, 29, 21, 13, 5,
	63, 55, 47, 39, 31, 23, 15, 7,
}
