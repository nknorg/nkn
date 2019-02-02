package pb

func rightShiftBytes(b []byte, n int) {
	for k := 0; k < n; k++ {
		b[len(b)-1] >>= 1
		for i := len(b) - 2; i >= 0; i-- {
			if b[i]&0x1 == 0x1 {
				b[i+1] |= 0x80
			}
			b[i] >>= 1
		}
	}
}
