package baker

import (
	"bytes"
	"io/ioutil"
	"os"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func MustReadFile(filename string) []byte {
	f, err := os.Open(filename)
	check(err)
	src, err := ioutil.ReadAll(f)
	check(err)
	return bytes.TrimSpace(src)
}
