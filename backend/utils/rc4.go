package utils

// RC4Xor meng-XOR data dengan keystream RC4 dan mengembalikan slice baru: KSA mengacak S dari key,
// lalu PRGA membangkitkan satu byte keystream per byte data (slide W5).
// drop byte keystream pertama dibuang dulu karena byte awal RC4 bias (RFC 4345 memakai 1536).
// Enkripsi dan dekripsi memakai fungsi yang sama. key harus 1 sampai 256 byte.
// Tipe byte sudah berputar balik di 255, jadi "mod 256" tidak perlu ditulis.
func RC4Xor(key, data []byte, drop int) []byte {
	// KSA
	var s [256]byte
	for n := range s {
		s[n] = byte(n)
	}
	var j byte
	for n := 0; n < 256; n++ {
		j += s[n] + key[n%len(key)]
		s[n], s[j] = s[j], s[n]
	}

	// PRGA
	var i byte
	j = 0
	next := func() byte {
		i++
		j += s[i]
		s[i], s[j] = s[j], s[i]
		return s[s[i]+s[j]]
	}
	for range drop {
		next()
	}

	output := make([]byte, len(data))
	for n, b := range data {
		output[n] = b ^ next()
	}
	return output
}
