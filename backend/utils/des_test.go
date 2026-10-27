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
