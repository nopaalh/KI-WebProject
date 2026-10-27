package utils

import (
	"bytes"
	"strconv"
	"testing"
)

func TestDESPermutation(t *testing.T) {
	// Panjang hasil mengikuti tabel (3), bukan input (4); posisi dihitung dari 1.
	got := DESPermutation([]byte{1, 0, 1, 1}, []byte{4, 2, 3})
	want := []byte{1, 0, 1}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDESSplit(t *testing.T) {
	plain := make([]byte, 64)
	for i := range plain {
		plain[i] = byte(i)
	}

	l0, r0 := DESSplit(plain)

	if len(l0) != 32 || len(r0) != 32 {
		t.Fatalf("panjang l0=%d r0=%d, want 32 dan 32", len(l0), len(r0))
	}
	if l0[0] != plain[0] || l0[31] != plain[31] {
		t.Fatal("l0 bukan 32 bit kiri dari plain")
	}
	if r0[0] != plain[32] || r0[31] != plain[63] {
		t.Fatal("r0 bukan 32 bit kanan dari plain")
	}

	// Hasil PC-1 (56 bit) harus terbelah 28 + 28 untuk C dan D.
	c0, d0 := DESSplit(make([]byte, 56))
	if len(c0) != 28 || len(d0) != 28 {
		t.Fatalf("panjang c0=%d d0=%d, want 28 dan 28", len(c0), len(d0))
	}
}

// bitsFromString mengubah teks "0101 ..." menjadi satu elemen per bit; selain 0 dan 1 diabaikan.
func bitsFromString(s string) []byte {
	var bits []byte
	for _, char := range s {
		if char == '0' || char == '1' {
			bits = append(bits, byte(char-'0'))
		}
	}
	return bits
}

func TestDESMerge(t *testing.T) {
	plain := make([]byte, 64)
	for i := range plain {
		plain[i] = byte(i)
	}

	l0, r0 := DESSplit(plain)
	merged := DESMerge(l0, r0)

	if !bytes.Equal(merged, plain) {
		t.Fatalf("merge(split(x)) = %v, want %v", merged, plain)
	}
	merged[0] = 99
	if l0[0] == 99 || plain[0] == 99 {
		t.Fatal("DESMerge berbagi memori dengan inputnya")
	}
}

func TestDESKeyScheduleFirstRoundKey(t *testing.T) {
	// Contoh kerja DES untuk kunci 133457799BBCDFF1: C0, D0, lalu K1 setelah rotasi 1 bit.
	key := bitsFromString("00010011 00110100 01010111 01111001 10011011 10111100 11011111 11110001")

	c, d := DESSplit(DESPermutation(key, desPC1[:]))
	if want := bitsFromString("1111000011001100101010101111"); !bytes.Equal(c, want) {
		t.Fatalf("C0 = %v, want %v", c, want)
	}
	if want := bitsFromString("0101010101100110011110001111"); !bytes.Equal(d, want) {
		t.Fatalf("D0 = %v, want %v", d, want)
	}

	k1 := DESPermutation(DESMerge(DESRotateLeft(c, 1), DESRotateLeft(d, 1)), desPC2[:])
	if want := bitsFromString("000110 110000 001011 101111 111111 000111 000001 110010"); !bytes.Equal(k1, want) {
		t.Fatalf("K1 = %v, want %v", k1, want)
	}
}

func TestDESSubKeyMaker(t *testing.T) {
	// Kunci 133457799BBCDFF1; ronde 9 adalah geser 1 bit pertama setelah deretan geser 2 bit.
	key := bitsFromString("00010011 00110100 01010111 01111001 10011011 10111100 11011111 11110001")
	want := map[int]string{
		1:  "000110 110000 001011 101111 111111 000111 000001 110010",
		2:  "011110 011010 111011 011001 110110 111100 100111 100101",
		8:  "111101 111000 101000 111010 110000 010011 101111 111011",
		9:  "111000 001101 101111 101011 111011 011110 011110 000001",
		16: "110010 110011 110110 001011 000011 100001 011111 110101",
	}

	subkeys := DESSubKeyMaker(key)

	for round, subkey := range subkeys {
		if len(subkey) != 48 {
			t.Fatalf("subkey ronde %d panjangnya %d, want 48", round+1, len(subkey))
		}
	}
	for round, bits := range want {
		if got := subkeys[round-1]; !bytes.Equal(got, bitsFromString(bits)) {
			t.Fatalf("K%d = %v, want %v", round, got, bitsFromString(bits))
		}
	}
}

// bitsFromHex mengubah 16 digit hex menjadi 64 elemen bit.
func bitsFromHex(s string) []byte {
	value, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		panic(err)
	}
	bits := make([]byte, 64)
	for i := range bits {
		bits[i] = byte(value >> (63 - i) & 1)
	}
	return bits
}

func TestDESFeistelTables(t *testing.T) {
	// desFP adalah kebalikan desIP.
	for _, hex := range []string{"0123456789ABCDEF", "FFFFFFFFFFFFFFFF", "8000000000000001"} {
		block := bitsFromHex(hex)
		if got := DESPermutation(DESPermutation(block, desIP[:]), desFP[:]); !bytes.Equal(got, block) {
			t.Fatalf("FP(IP(%s)) = %v", hex, got)
		}
	}

	// desP memakai tiap posisi 1..32 tepat sekali.
	seen := make(map[byte]bool)
	for _, position := range desP {
		if position < 1 || position > 32 || seen[position] {
			t.Fatalf("desP: posisi %d di luar rentang atau berulang", position)
		}
		seen[position] = true
	}

	// desE: bit tepi tiap kelompok 4 (posisi 4n dan 4n+1) muncul dua kali, sisanya sekali.
	count := make(map[byte]int)
	for _, position := range desE {
		count[position]++
	}
	for position := byte(1); position <= 32; position++ {
		want := 1
		if position%4 == 0 || position%4 == 1 {
			want = 2
		}
		if count[position] != want {
			t.Fatalf("desE: posisi %d muncul %d kali, want %d", position, count[position], want)
		}
	}

	// Tiap baris S-box adalah permutasi 0..15.
	for box := range desSBoxes {
		for row := range desSBoxes[box] {
			used := [16]bool{}
			for _, value := range desSBoxes[box][row] {
				if value > 15 || used[value] {
					t.Fatalf("S%d baris %d bukan permutasi 0..15", box+1, row)
				}
				used[value] = true
			}
		}
	}
}

func TestDESBitsAndInt(t *testing.T) {
	if got := DESBitsToInt([]byte{1, 1, 0, 0}); got != 12 {
		t.Fatalf("DESBitsToInt(1100) = %d, want 12", got)
	}
	if got := DESIntToBits(12, 4); !bytes.Equal(got, []byte{1, 1, 0, 0}) {
		t.Fatalf("DESIntToBits(12, 4) = %v, want [1 1 0 0]", got)
	}
	// Lebar lebih besar dari panjang bit angka diisi nol di kiri.
	if got := DESIntToBits(5, 6); !bytes.Equal(got, []byte{0, 0, 0, 1, 0, 1}) {
		t.Fatalf("DESIntToBits(5, 6) = %v, want [0 0 0 1 0 1]", got)
	}
}

func TestDESFeistelFunction(t *testing.T) {
	// Contoh kerja FIPS ronde 1: R0 dan K1 menghasilkan F = 234AA9BB.
	r0 := bitsFromString("1111 0000 1010 1010 1111 0000 1010 1010")
	k1 := bitsFromString("000110 110000 001011 101111 111111 000111 000001 110010")

	got := DESFeistelFunction(r0, k1)

	if want := bitsFromString("0010 0011 0100 1010 1010 1001 1011 1011"); !bytes.Equal(got, want) {
		t.Fatalf("F(R0, K1) = %v, want %v", got, want)
	}
}

func TestDESRotateLeft(t *testing.T) {
	bits := []byte{1, 2, 3, 4}
	original := bytes.Clone(bits)

	for shift, want := range map[int][]byte{1: {2, 3, 4, 1}, 2: {3, 4, 1, 2}} {
		if got := DESRotateLeft(bits, shift); !bytes.Equal(got, want) {
			t.Fatalf("shift %d: got %v, want %v", shift, got, want)
		}
	}
	if !bytes.Equal(bits, original) {
		t.Fatal("DESRotateLeft mengubah input")
	}
}

func TestDESPermutedChoiceTables(t *testing.T) {
	// PC-1: 56 posisi berbeda dari 64, tanpa bit paritas (kelipatan 8).
	seen := make(map[byte]bool)
	for _, position := range desPC1 {
		if position < 1 || position > 64 || position%8 == 0 || seen[position] {
			t.Fatalf("desPC1: posisi %d di luar rentang, paritas, atau berulang", position)
		}
		seen[position] = true
	}

	// PC-2: 48 posisi berbeda dari 56, tanpa 8 bit yang dibuang FIPS 46-3.
	dropped := map[byte]bool{9: true, 18: true, 22: true, 25: true, 35: true, 38: true, 43: true, 54: true}
	seen = make(map[byte]bool)
	for _, position := range desPC2 {
		if position < 1 || position > 56 || dropped[position] || seen[position] {
			t.Fatalf("desPC2: posisi %d di luar rentang, dibuang, atau berulang", position)
		}
		seen[position] = true
	}
}

func TestDESPermutationInitialPermutation(t *testing.T) {
	input := make([]byte, 64)
	input[0] = 1 // hanya bit 1 yang menyala

	got := DESPermutation(input, desIP[:])

	// desIP menaruh bit input 1 di posisi output 40 (baris 5, kolom 8).
	for i, bit := range got {
		if i != 39 && bit != 0 {
			t.Fatalf("bit output %d menyala, seharusnya hanya bit 40", i+1)
		}
	}
	if got[39] != 1 {
		t.Fatal("bit input 1 tidak sampai di posisi output 40")
	}
}
