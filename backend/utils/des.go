package utils

import "slices"

// utility function
// DESPermutation
func DESPermutation(input []byte, table []byte) []byte {
	output := make([]byte, len(table))
	for i := 0; i < len(table); i++ {
		output[i] = input[table[i]-1]
	}
	return output
}

// DESSplit membelah bit menjadi dua bagian sama panjang: 64 bit jadi l0 dan r0 (32 + 32),
// 56 bit hasil PC-1 jadi c0 dan d0 (28 + 28).
func DESSplit(plain []byte) (l0, r0 []byte) {
	half := len(plain) / 2
	l0 = plain[:half]
	r0 = plain[half:]

	return l0, r0
}

// DESMerge menggabungkan c (kiri) dan d (kanan) menjadi satu slice baru.
// Kebalikan DESSplit, tapi hasilnya tidak berbagi memori dengan c dan d.
func DESMerge(c, d []byte) []byte {
	return slices.Concat(c, d)
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

// Key untilScheduling
//
// desPC1 fungsinya buat ngebuang paritas (ngecek validitas key)
var desPC1 = [56]byte{
	57, 49, 41, 33, 25, 17, 9,
	1, 58, 50, 42, 34, 26, 18,
	10, 2, 59, 51, 43, 35, 27,
	19, 11, 3, 60, 52, 44, 36,
	63, 55, 47, 39, 31, 23, 15,
	7, 62, 54, 46, 38, 30, 22,
	14, 6, 61, 53, 45, 37, 29,
	21, 13, 5, 28, 20, 12, 4,
}

// Setelahitu Split desPC1 jadi dua dengan masing masing half 28 bit
// Gunakan DESSplit
// Setelahitu Lanjut gunakan DESRotateLeft buat C sama D
// Lanjut gabungin C sama D pake DESMerge
// Lanjut declare desPC2 basically randomizer sama kayak IP cuman ya versi buat Key
var desPC2 = [48]byte{
	14, 17, 11, 24, 1, 5,
	3, 28, 15, 6, 21, 10,
	23, 19, 12, 4, 26, 8,
	16, 7, 27, 20, 13, 2,
	41, 52, 31, 37, 47, 55,
	30, 40, 51, 45, 33, 48,
	44, 49, 39, 56, 34, 53,
	46, 42, 50, 36, 29, 32,
}

// pake desPC2 buat ngereduksi supaya yang awalnya C||D 56 bit bisa jadi 48
// Done!
