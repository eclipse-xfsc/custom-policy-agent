package err

import (
	"crypto/rand"
	"fmt"
)

const Alphabet string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

func NewID() string {
	length := 16

	ll := len(Alphabet)
	b := make([]byte, length)
	_, err := rand.Read(b) // generates len(b) random bytes
	if err != nil {
		panic(fmt.Errorf("failed to read random bytes: %v", err))
	}
	for i := 0; i < length; i++ {
		b[i] = Alphabet[int(b[i])%ll]
	}

	return string(b)
}
