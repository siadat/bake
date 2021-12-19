package main

import (
	"fmt"
	"strings"
	"text/scanner"
)

func main() {
	src := `
fn greet(name string) begin
  printf("hello %s!\n", name)
end

fn main() begin
  greet("GopherCon")
end
`
	var s scanner.Scanner
	s.Init(strings.NewReader(src))
	for tok := s.Scan(); tok != scanner.EOF; tok = s.Scan() {
		fmt.Printf("%s: \t %s\n", s.Position, s.TokenText())
	}
}
