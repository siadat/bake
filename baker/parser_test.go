package baker_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/maxatome/go-testdeep/td"
	"github.com/siadat/bake/baker"
)

func TestParser(tt *testing.T) {
	t := td.Assert(tt)
	bakerSrcReader := strings.NewReader(`
	fn greet(name string) begin
	  printf("hello %s!\n", name)
	end

	fn main() begin
	  greet("GopherCon")
	end
	`)
	goSrcReader := strings.NewReader(`
	package main

	import "fmt"

	func greet(name string) {
			fmt.Printf("hello %s!\n", name)
	}

	func main() {
			greet("GopherCon")
	}
	`)

	prsr := baker.NewParser("test.bake", bakerSrcReader)
	gotFile := prsr.ParseFile()

	wantFset := token.NewFileSet()
	wantFile, err := parser.ParseFile(wantFset, "test.bake", goSrcReader, 0)
	t.CmpNoError(err)

	{
		// ignore some fields, so that we could use t.Cmp
		t = t.WithCmpHooks(func(token.Pos, token.Pos) error { return nil })     // ignoring just to simplify the comparison
		t = t.WithCmpHooks(func(*ast.Object, *ast.Object) error { return nil }) // not used
		wantFile.Scope = nil                                                    // not used
		wantFile.Unresolved = nil                                               // not used
		wantFile.Imports = nil                                                  // imports are added in wantFile.Decls
	}

	if !t.Cmp(gotFile, wantFile) {
		ast.Print(prsr.FileSet, gotFile)
		t.FailNow()
	}
}
