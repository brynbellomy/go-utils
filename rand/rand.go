package rand

import (
	"math/rand"
	"strconv"

	"github.com/google/uuid"
)

func RandomNumberString() string {
	return strconv.Itoa(rand.Intn(8999) + 1000)
}

func RandomBytesUnsafe(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func RandomString(n int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		idx := rand.Intn(len(charset))
		b[i] = charset[idx]
	}
	return string(b), nil
}

func MustUUIDv7() string {
	vid, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return vid.String()
}
