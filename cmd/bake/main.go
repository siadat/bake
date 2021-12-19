package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/printer"
	"go/scanner"
	"go/token"
	"go/types"
	"os"

	"github.com/siadat/bake/baker"
)

var verbose bool

func init() {
	flag.BoolVar(&verbose, "v", false, "verbose")
}

func main() {
	flag.Parse()
	filename := flag.Arg(0)

	reader, err := os.Open(filename)
	if err != nil {
		panic(err)
	}

	prsr := baker.NewParser(filename, reader)
	fset := prsr.FileSet
	got := prsr.ParseFile()

	if verbose {
		ast.Print(fset, got)
	}

	{
		file := got
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			if genDecl.Tok != token.TYPE {
				continue
			}
			typeSpec, ok := genDecl.Specs[0].(*ast.TypeSpec)
			if !ok {
				continue
			}
			unionSpec, ok := typeSpec.Type.(*baker.UnionType)
			if !ok {
				continue
			}
			name := typeSpec.Name.Name
			fmt.Printf("typeSpec %q: %T\n", name, unionSpec)

			var typesList []ast.Expr
			for _, typ := range unionSpec.Types {
				typesList = append(typesList, typ)
			}

			// replace:
			typeSpec.Type = &ast.InterfaceType{
				Interface: unionSpec.Union,
				Methods:   &ast.FieldList{}, // empty
			}

			// add a helper type
			{
				var list []*ast.Field
				// the type itself
				list = append(list, &ast.Field{
					Names: []*ast.Ident{
						{
							Name: "Type",
						},
					},
					Type: &ast.Ident{Name: name},
				})
				// its possible types
				for i, typ := range typesList {
					list = append(list, &ast.Field{
						Names: []*ast.Ident{
							{
								Name: fmt.Sprintf("Field%d", i),
							},
						},
						Type: typ,
					})
				}

				file.Decls = append(file.Decls, &ast.GenDecl{
					Tok: token.TYPE,
					Specs: []ast.Spec{
						&ast.TypeSpec{
							Name: &ast.Ident{Name: "__" + name},
							Type: &ast.StructType{
								Fields: &ast.FieldList{
									List: list,
								},
							},
						},
					},
				})
			}
		}
	}

	// go type checker
	printer.Fprint(os.Stdout, fset, got)
	typeCheck(fset, got)
}

func typeCheck(fset *token.FileSet, astFile *ast.File) *types.Info {
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}
	conf := &types.Config{
		Error:    typecheckError,
		Importer: importer.Default(),
	}
	_, err := conf.Check("", fset, []*ast.File{astFile}, info)
	if err != nil {
		fmt.Println(err)
		return info
	}
	return info
}

func typecheckError(err error) {
	switch err := err.(type) {
	case scanner.ErrorList:
		for _, e := range err {
			typecheckError(e)
		}

	case types.Error:
		fmt.Printf("typecheckError: %+v\n", err)

	default:
		panic(fmt.Sprintf("typecheck %T", err))
	}
}
