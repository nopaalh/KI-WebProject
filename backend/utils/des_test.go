package utils

import (
	"bytes"
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
