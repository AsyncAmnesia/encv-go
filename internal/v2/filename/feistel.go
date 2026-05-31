package filename

func FeistelEncrypt(data []byte, sbox *SBox, roundKeys [][]byte) []byte {
	if len(data) == 0 {
		return data
	}
	n := len(data)
	padded := padToEven(data)

	left := padded[:n/2]
	right := padded[n/2:]

	for i, key := range roundKeys {
		f := roundFunc(right, key, sbox)
		for j := range left {
			left[j] ^= f[j]
		}
		if i < len(roundKeys)-1 {
			left, right = right, left
		}
	}

	result := make([]byte, n)
	copy(result, left)
	copy(result[len(left):], right)
	return result
}

func FeistelDecrypt(data []byte, sbox *SBox, roundKeys [][]byte) []byte {
	if len(data) == 0 {
		return data
	}
	n := len(data)
	padded := padToEven(data)

	left := padded[:n/2]
	right := padded[n/2:]

	for i := len(roundKeys) - 1; i >= 0; i-- {
		f := roundFunc(right, roundKeys[i], sbox)
		for j := range left {
			left[j] ^= f[j]
		}
		if i > 0 {
			left, right = right, left
		}
	}

	result := make([]byte, n)
	copy(result, left)
	copy(result[len(left):], right)
	return result
}

func roundFunc(half, key []byte, sbox *SBox) []byte {
	out := make([]byte, len(half))
	minLen := len(half)
	if len(key) < minLen {
		minLen = len(key)
	}
	for i := 0; i < minLen; i++ {
		out[i] = sbox.Forward[(int(half[i])+int(key[i]))&0xFF]
	}
	for i := minLen; i < len(out); i++ {
		out[i] = sbox.Forward[half[i]]
	}
	return out
}

func padToEven(data []byte) []byte {
	if len(data)%2 == 0 {
		return data
	}
	return append(data, 0)
}
