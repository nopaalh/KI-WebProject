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

// DESSubKeyMaker buat gabungin dan iterasi tiap ronde.
// key berisi 64 bit , hasilnya 16 subkey masing-masing 48 bit.
func DESSubKeyMaker(key []byte) [16][]byte {
	var subkeys [16][]byte
	c, d := DESSplit(DESPermutation(key, desPC1[:]))

	for round := 1; round <= 16; round++ {
		shift := 2
		switch round {
		case 1, 2, 9, 16:
			shift = 1
		}
		// c dan d diputar terus dari hasil ronde sebelumnya, bukan dari c0 dan d0.
		c = DESRotateLeft(c, shift)
		d = DESRotateLeft(d, shift)
		subkeys[round-1] = DESPermutation(DESMerge(c, d), desPC2[:])
	}
	return subkeys
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

// Feistel
//
// desE melebarkan R dari 32 bit jadi 48 bit (bit tepi tiap kelompok 4 diulang) supaya bisa di-XOR dengan subkey.
var desE = [48]byte{
	32, 1, 2, 3, 4, 5,
	4, 5, 6, 7, 8, 9,
	8, 9, 10, 11, 12, 13,
	12, 13, 14, 15, 16, 17,
	16, 17, 18, 19, 20, 21,
	20, 21, 22, 23, 24, 25,
	24, 25, 26, 27, 28, 29,
	28, 29, 30, 31, 32, 1,
}

// desSBoxes mereduksi tiap kelompok 6 bit jadi 4 bit: baris dari bit pertama dan terakhir,
// kolom dari empat bit tengah.
var desSBoxes = [8][4][16]byte{
	{
		{14, 4, 13, 1, 2, 15, 11, 8, 3, 10, 6, 12, 5, 9, 0, 7},
		{0, 15, 7, 4, 14, 2, 13, 1, 10, 6, 12, 11, 9, 5, 3, 8},
		{4, 1, 14, 8, 13, 6, 2, 11, 15, 12, 9, 7, 3, 10, 5, 0},
		{15, 12, 8, 2, 4, 9, 1, 7, 5, 11, 3, 14, 10, 0, 6, 13},
	},
	{
		{15, 1, 8, 14, 6, 11, 3, 4, 9, 7, 2, 13, 12, 0, 5, 10},
		{3, 13, 4, 7, 15, 2, 8, 14, 12, 0, 1, 10, 6, 9, 11, 5},
		{0, 14, 7, 11, 10, 4, 13, 1, 5, 8, 12, 6, 9, 3, 2, 15},
		{13, 8, 10, 1, 3, 15, 4, 2, 11, 6, 7, 12, 0, 5, 14, 9},
	},
	{
		{10, 0, 9, 14, 6, 3, 15, 5, 1, 13, 12, 7, 11, 4, 2, 8},
		{13, 7, 0, 9, 3, 4, 6, 10, 2, 8, 5, 14, 12, 11, 15, 1},
		{13, 6, 4, 9, 8, 15, 3, 0, 11, 1, 2, 12, 5, 10, 14, 7},
		{1, 10, 13, 0, 6, 9, 8, 7, 4, 15, 14, 3, 11, 5, 2, 12},
	},
	{
		{7, 13, 14, 3, 0, 6, 9, 10, 1, 2, 8, 5, 11, 12, 4, 15},
		{13, 8, 11, 5, 6, 15, 0, 3, 4, 7, 2, 12, 1, 10, 14, 9},
		{10, 6, 9, 0, 12, 11, 7, 13, 15, 1, 3, 14, 5, 2, 8, 4},
		{3, 15, 0, 6, 10, 1, 13, 8, 9, 4, 5, 11, 12, 7, 2, 14},
	},
	{
		{2, 12, 4, 1, 7, 10, 11, 6, 8, 5, 3, 15, 13, 0, 14, 9},
		{14, 11, 2, 12, 4, 7, 13, 1, 5, 0, 15, 10, 3, 9, 8, 6},
		{4, 2, 1, 11, 10, 13, 7, 8, 15, 9, 12, 5, 6, 3, 0, 14},
		{11, 8, 12, 7, 1, 14, 2, 13, 6, 15, 0, 9, 10, 4, 5, 3},
	},
	{
		{12, 1, 10, 15, 9, 2, 6, 8, 0, 13, 3, 4, 14, 7, 5, 11},
		{10, 15, 4, 2, 7, 12, 9, 5, 6, 1, 13, 14, 0, 11, 3, 8},
		{9, 14, 15, 5, 2, 8, 12, 3, 7, 0, 4, 10, 1, 13, 11, 6},
		{4, 3, 2, 12, 9, 5, 15, 10, 11, 14, 1, 7, 6, 0, 8, 13},
	},
	{
		{4, 11, 2, 14, 15, 0, 8, 13, 3, 12, 9, 7, 5, 10, 6, 1},
		{13, 0, 11, 7, 4, 9, 1, 10, 14, 3, 5, 12, 2, 15, 8, 6},
		{1, 4, 11, 13, 12, 3, 7, 14, 10, 15, 6, 8, 0, 5, 9, 2},
		{6, 11, 13, 8, 1, 4, 10, 7, 9, 5, 0, 15, 14, 2, 3, 12},
	},
	{
		{13, 2, 8, 4, 6, 15, 11, 1, 10, 9, 3, 14, 5, 0, 12, 7},
		{1, 15, 13, 8, 10, 3, 7, 4, 12, 5, 6, 11, 0, 14, 9, 2},
		{7, 11, 4, 1, 9, 12, 14, 2, 0, 6, 10, 13, 15, 3, 5, 8},
		{2, 1, 14, 7, 4, 10, 8, 13, 15, 12, 9, 0, 3, 5, 6, 11},
	},
}

// desP mengacak 32 bit hasil S-box, logikanya sama seperti desIP.
var desP = [32]byte{
	16, 7, 20, 21, 29, 12, 28, 17,
	1, 15, 23, 26, 5, 18, 31, 10,
	2, 8, 24, 14, 32, 27, 3, 9,
	19, 13, 30, 6, 22, 11, 4, 25,
}

// desFP adalah kebalikan desIP, dipakai setelah R16 dan L16 digabung.
var desFP = [64]byte{
	40, 8, 48, 16, 56, 24, 64, 32,
	39, 7, 47, 15, 55, 23, 63, 31,
	38, 6, 46, 14, 54, 22, 62, 30,
	37, 5, 45, 13, 53, 21, 61, 29,
	36, 4, 44, 12, 52, 20, 60, 28,
	35, 3, 43, 11, 51, 19, 59, 27,
	34, 2, 42, 10, 50, 18, 58, 26,
	33, 1, 41, 9, 49, 17, 57, 25,
}

// DESXor meng-XOR dua slice bit elemen per elemen dan mengembalikan slice baru.
func DESXor(a, b []byte) []byte {
	output := make([]byte, len(a))
	for i := range a {
		output[i] = a[i] ^ b[i]
	}
	return output
}

// desSBoxRow dan desSBoxColumn memilih bit baris dan kolom dari kelompok 6 bit,
// logikanya sama seperti desIP: baris dari bit ke-1 dan ke-6, kolom dari bit ke-2 sampai ke-5.
var desSBoxRow = []byte{1, 6}
var desSBoxColumn = []byte{2, 3, 4, 5}

// DESBitsToInt mengubah bit (kiri = paling besar) jadi angka, misalnya 1100 jadi 12.
func DESBitsToInt(bits []byte) int {
	number := 0
	for _, bit := range bits {
		number = number*2 + int(bit)
	}
	return number
}

// DESIntToBits kebalikan DESBitsToInt -> angka jadi width bit, misalnya 12 dengan width 4 jadi 1100.
func DESIntToBits(number, width int) []byte {
	bits := make([]byte, width)
	for i := width - 1; i >= 0; i-- {
		bits[i] = byte(number % 2)
		number /= 2
	}
	return bits
}

// DESBytesToBits mengubah bytes jadi bit, 8 bit per byte dengan bit terbesar di kiri.
// 8 byte jadi 64 bit, siap dipakai DESSubKeyMaker dan DESCrypt.
func DESBytesToBits(data []byte) []byte {
	bits := make([]byte, 0, len(data)*8)
	for _, b := range data {
		bits = append(bits, DESIntToBits(int(b), 8)...)
	}
	return bits
}

// DESBitsToBytes kebalikan DESBytesToBits; panjang bits harus kelipatan 8.
func DESBitsToBytes(bits []byte) []byte {
	data := make([]byte, len(bits)/8)
	for i := range data {
		data[i] = byte(DESBitsToInt(bits[i*8 : i*8+8]))
	}
	return data
}

// DESSBoxSubstitution mereduksi 48 bit jadi 32 bit, tiap kelompok 6 bit masuk S-box miliknya sendiri
// dan keluar sebagai 4 bit, lalu semuanya digabung berurutan.
func DESSBoxSubstitution(input []byte) []byte {
	output := make([]byte, 0, 32)
	for box := 0; box < 8; box++ {
		group := input[box*6 : box*6+6]
		row := DESBitsToInt(DESPermutation(group, desSBoxRow))
		column := DESBitsToInt(DESPermutation(group, desSBoxColumn))
		value := desSBoxes[box][row][column]
		output = append(output, DESIntToBits(int(value), 4)...)
	}
	return output
}

// DESFeistelFunction adalah fungsi F: ekspansi desE, XOR subkey, S-box, lalu desP.
func DESFeistelFunction(r, subkey []byte) []byte {
	expanded := DESPermutation(r, desE[:])
	mixed := DESXor(expanded, subkey)
	return DESPermutation(DESSBoxSubstitution(mixed), desP[:])
}

// DESCrypt memproses satu blok 64 bit, desIP, split, 16 ronde, gabung R16||L16, lalu desFP.
// Setiap ronde -> L baru = R lama, R baru = L lama XOR F(R lama, subkey ronde itu).
// Untuk dekripsi, kirim subkeys dalam urutan terbalik.
func DESCrypt(block []byte, subkeys [16][]byte) []byte {
	l, r := DESSplit(DESPermutation(block, desIP[:]))
	for round := 0; round < 16; round++ {
		l, r = r, DESXor(l, DESFeistelFunction(r, subkeys[round]))
	}
	return DESPermutation(DESMerge(r, l), desFP[:])
}
