package main

import (
	"go/ast"
	"go/parser"
	"go/token"
)

func main() {
	src := "3 + 2"
	fset := token.NewFileSet()
	expr, err := parser.ParseExprFrom(fset, "", []byte(src), 0)
	if err != nil {
		panic(err)
	}

	ast.Print(fset, expr)
}
