package utils

import (
	"math/rand"
	"strconv"

	"github.com/google/uuid"
)

func RandomNumberString() string {
	return strconv.Itoa(rand.Intn(8999) + 1000)
}

func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func RandomString(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
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
