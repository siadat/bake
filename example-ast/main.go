package main

import (
	"go/ast"
	"go/parser"
	"go/token"
)

func main() {
	expr, _ := parser.ParseExpr("3 + 2")
	ast.Print(token.NewFileSet(), expr)
}
