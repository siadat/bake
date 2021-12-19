package baker

import (
	"bytes"
	"fmt"
	"text/scanner"
)

type GoScanner struct {
	scnr scanner.Scanner
}

func (s *GoScanner) Init(filename string, src []byte) {
	s.scnr.Filename = filename
	s.scnr.Init(bytes.NewReader(src))
	s.scnr.Whitespace ^= 1<<'\n' | 1<<'\r' // don't skip newlines
}

func (s *GoScanner) SetSkipUntil(enders ...rune) {
	if len(enders) == 0 {
		s.scnr.IsIdentRune = nil
		return
	}

	s.scnr.IsIdentRune = func(ch rune, i int) bool {
		for _, ender := range enders {
			if ch == ender {
				return false
			}
		}
		return true
	}
}

func (s *GoScanner) log(lit string) {
	fmt.Printf("%s \t lit:%q \n",
		s.scnr.Position,
		lit,
	)
}
