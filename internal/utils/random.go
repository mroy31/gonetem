package utils

import (
	"math/rand"
	"time"
)

var (
	isInit      = false
	letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
)

func initRand() {
	rand.Seed(time.Now().UnixNano())
}

func RandString(n int) string {
	if !isInit {
		initRand()
		isInit = true
	}
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}
