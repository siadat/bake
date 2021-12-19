package baker

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"text/scanner"
)

const (
	KeywordSemicolon = ";"

	KeywordPkg = "pkg"
	KeywordFn  = "fn"

	KeywordBegin  = "begin"
	KeywordEnd    = "end"
	KeywordPrintf = "printf"

	KeywordImport    = "import"
	KeywordVar       = "var"
	KeywordReturn    = "return"
	KeywordType      = "type"
	KeywordSwitch    = "switch"
	KeywordCase      = "case"
	KeywordIf        = "if"
	KeywordStruct    = "struct"
	KeywordUnion     = "union"
	KeywordInterface = "interface"
)

type Parser struct {
	GoScanner GoScanner
	FileSet   *token.FileSet
	verbose   bool

	File *ast.File

	Curr struct {
		Tok rune
		Lit string
	}
}

func NewParser(filename string, reader io.Reader) *Parser {
	f, err := ioutil.ReadAll(reader)
	check(err)
	src := bytes.TrimSpace(f)

	p := &Parser{
		FileSet: token.NewFileSet(),
		File:    &ast.File{},
	}

	fsetFile := p.FileSet.AddFile(filename, -1, len(src))
	fsetFile.SetLinesForContent(src)

	p.verbose = false
	p.GoScanner.Init(filename, src)
	p.next()
	return p
}

func (p *Parser) parseIdent() *ast.Ident {
	lit := p.Curr.Lit
	if p.Curr.Tok != scanner.Ident {
		panic("oh no")
	}
	p.next()
	return &ast.Ident{
		NamePos: token.Pos(p.GoScanner.scnr.Position.Offset + 1),
		Name:    lit,
	}
}

func (p *Parser) parseString() (string, scanner.Position) {
	lit := p.Curr.Lit
	if p.Curr.Tok != scanner.String {
		panic("oh no")
	}
	p.next()
	return lit, p.GoScanner.scnr.Position
}

func (p *Parser) fromHereToEndOfLine() string {
	str := ""
	p.GoScanner.SetSkipUntil('\n', '\r', ';')
	for p.Curr.Tok != scanner.EOF && p.Curr.Lit != ";" {
		str += p.Curr.Lit
		p.next()
	}
	p.GoScanner.SetSkipUntil()
	return str
}

func (p *Parser) parseValue(pos scanner.Position, str string) ast.Expr {
	expr, err := parser.ParseExpr(str)
	if err != nil {
		panic(fmt.Sprintf("Parsing %q failed: %v", str, err))
	}
	switch expr := expr.(type) {
	case *ast.BasicLit:
		expr.ValuePos = token.Pos(pos.Offset + int(expr.ValuePos))
	}
	return expr
}

func (p *Parser) parseValueToEndOfLine() ast.Expr {
	pos := p.GoScanner.scnr.Position
	str := p.fromHereToEndOfLine()
	expr := p.parseValue(pos, str)
	p.expectSemicolon()
	return expr
}

func (p *Parser) parseMethodSpec() *ast.Field {
	ident := p.parseIdent()
	params := p.parseFieldList()
	results := p.parseTypeList()
	p.expectSemicolon()

	return &ast.Field{
		Names: []*ast.Ident{ident},
		Type: &ast.FuncType{
			Params:  params,
			Results: results,
		},
	}
}

func (p *Parser) parseInterfaceType() *ast.InterfaceType {
	interfacePos := p.expectLit(KeywordInterface)
	lbrace := p.GoScanner.scnr.Position
	p.expectLit(KeywordBegin)

	var list []*ast.Field
	for {
		p.skipNewLines()
		if p.Curr.Lit == KeywordEnd {
			break
		}
		list = append(list, p.parseMethodSpec())
	}

	rbrace := p.GoScanner.scnr.Position
	p.expectLit(KeywordEnd)

	return &ast.InterfaceType{
		Interface: token.Pos(interfacePos.Offset + 1),
		Methods: &ast.FieldList{
			Opening: token.Pos(lbrace.Offset + 1),
			List:    list,
			Closing: token.Pos(rbrace.Offset + 1),
		},
	}
}

type UnionType struct {
	ast.Expr

	Union token.Pos
	Types []ast.Expr
}

func (u *UnionType) Pos() token.Pos { return u.Union }
func (u *UnionType) End() token.Pos { return u.Union }

func (p *Parser) parseUnionType() *UnionType {
	pos := p.expectLit(KeywordUnion)
	p.expectLit("=")

	var list []ast.Expr
	for p.Curr.Tok != scanner.EOF && p.Curr.Lit != ";" {
		typ := p.parseType()
		list = append(list, typ)
		if p.Curr.Lit == "|" {
			p.expectLit("|")
		}
	}

	return &UnionType{
		Union: token.Pos(pos.Offset + 1),
		Types: list,
	}
}

func (p *Parser) parseStructType() *ast.StructType {
	structPos := p.expectLit(KeywordStruct)
	lbrace := p.GoScanner.scnr.Position
	p.expectLit(KeywordBegin)

	var list []*ast.Field
	for {
		p.skipNewLines()
		if p.Curr.Lit == KeywordEnd {
			break
		}
		list = append(list, p.parseArgParam())
	}

	rbrace := p.GoScanner.scnr.Position
	p.expectLit(KeywordEnd)

	return &ast.StructType{
		Struct: token.Pos(structPos.Offset + 1),
		Fields: &ast.FieldList{
			Opening: token.Pos(lbrace.Offset + 1),
			List:    list,
			Closing: token.Pos(rbrace.Offset + 1),
		},
	}
}

func (p *Parser) parseType() ast.Expr {
	switch p.Curr.Lit {
	case "*":
		star := p.GoScanner.scnr.Position
		p.next()
		return &ast.StarExpr{
			Star: token.Pos(star.Offset + 1),
			X:    p.parseType(),
		}
	case KeywordStruct:
		return p.parseStructType()
	case KeywordUnion:
		return p.parseUnionType()
	case KeywordInterface:
		return p.parseInterfaceType()
	default:
		return p.parseIdent()
	}
}

func (p *Parser) parseRetParam() *ast.Field {
	typ := p.parseType()
	return &ast.Field{
		Names: nil,
		Type:  typ,
	}
}

func (p *Parser) parseArgParam() *ast.Field {
	ident := p.parseIdent()
	typ := p.parseType()

	return &ast.Field{
		Names: []*ast.Ident{ident},
		Type:  typ,
	}
}

func (p *Parser) parseFieldList() *ast.FieldList {
	var params []*ast.Field

	lparen := p.GoScanner.scnr.Position
	p.expectLit("(")

	for {
		if p.Curr.Lit == ")" {
			break
		}
		params = append(params, p.parseArgParam())
		if p.Curr.Lit == "," {
			p.next()
		}
	}

	rparen := p.GoScanner.scnr.Position
	p.expectLit(")")

	return &ast.FieldList{
		Opening: token.Pos(lparen.Offset + 1),
		List:    params,
		Closing: token.Pos(rparen.Offset + 1),
	}
}

func (p *Parser) parseTypeList() *ast.FieldList {
	var params []*ast.Field

	lparen := p.GoScanner.scnr.Position
	p.expectLit("(")

	for {
		if p.Curr.Lit == ")" {
			break
		}
		params = append(params, p.parseRetParam())
		if p.Curr.Lit == "," {
			p.next()
		}
	}

	rparen := p.GoScanner.scnr.Position
	p.expectLit(")")

	return &ast.FieldList{
		Opening: token.Pos(lparen.Offset + 1),
		List:    params,
		Closing: token.Pos(rparen.Offset + 1),
	}
}

func (p *Parser) parseReturnStmt() *ast.ReturnStmt {
	pos := p.expectLit(KeywordReturn)
	return &ast.ReturnStmt{
		Return:  token.Pos(pos.Offset + 1),
		Results: []ast.Expr{p.parseValueToEndOfLine()},
	}
}

func (p *Parser) parseCaseClause() *ast.CaseClause {
	var pos scanner.Position
	var exprs []ast.Expr
	var colon scanner.Position

	isDefault := false
	switch p.Curr.Lit {
	case "case":
		pos = p.GoScanner.scnr.Position
		p.next()
	case "default":
		pos = p.GoScanner.scnr.Position
		p.next()
		colon = p.GoScanner.scnr.Position
		p.next()
		isDefault = true
	default:
		panic("wat")
	}

	if !isDefault {
		exprPos := p.GoScanner.scnr.Position
		str := p.fromHereToEndOfLine()
		exprs = append(exprs, p.parseValue(exprPos, str[:len(str)-1]))
		p.expectSemicolon()
		colon = p.GoScanner.scnr.Position
	}

	body := p.parseStmtsUntil(KeywordEnd, "case", "default")

	return &ast.CaseClause{
		Case:  token.Pos(pos.Offset + 1),
		List:  exprs,
		Colon: token.Pos(colon.Offset + 1),
		Body:  body,
	}
}

func (p *Parser) parseSwitchBody() *ast.BlockStmt {
	lbrace := p.GoScanner.scnr.Position

	var list []ast.Stmt
	for p.Curr.Lit == "case" || p.Curr.Lit == "default" {
		list = append(list, p.parseCaseClause())
	}

	rbrace := p.GoScanner.scnr.Position
	p.expectLit(KeywordEnd)

	return &ast.BlockStmt{
		Lbrace: token.Pos(lbrace.Offset + 1),
		List:   list,
		Rbrace: token.Pos(rbrace.Offset + 1),
	}
}

func (p *Parser) parseExpr() ast.Expr {
	pos := p.GoScanner.scnr.Position
	var expr ast.Expr

	switch p.Curr.Tok {
	case scanner.Int:
		expr = &ast.BasicLit{
			ValuePos: token.Pos(pos.Offset + 1),
			Kind:     token.INT,
			Value:    p.Curr.Lit,
		}
	case scanner.Float:
		expr = &ast.BasicLit{
			ValuePos: token.Pos(pos.Offset + 1),
			Kind:     token.FLOAT,
			Value:    p.Curr.Lit,
		}
	case scanner.String:
		expr = &ast.BasicLit{
			ValuePos: token.Pos(pos.Offset + 1),
			Kind:     token.STRING,
			Value:    p.Curr.Lit,
		}
	case scanner.Ident:
		expr = &ast.Ident{
			NamePos: token.Pos(pos.Offset + 1),
			Name:    p.Curr.Lit,
		}
	default:
		panic("oh no")
	}
	p.next()
	return expr
}

func (p *Parser) addImportDelc(path string) {

	for i := range p.File.Decls {
		decl, ok := p.File.Decls[i].(*ast.GenDecl)
		if !ok {
			continue
		}
		if decl.Tok != token.IMPORT {
			continue
		}

		spec, ok := decl.Specs[0].(*ast.ImportSpec)
		if !ok {
			continue
		}

		if spec.Path.Value == fmt.Sprintf("%q", path) {
			// found
			return
		}
	}

	p.File.Decls = append(p.File.Decls, &ast.GenDecl{
		Tok: token.IMPORT,
		Specs: []ast.Spec{
			&ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: fmt.Sprintf("%q", path),
				},
			},
		},
	})
}

func (p *Parser) parseCallExpr() *ast.CallExpr {
	var fn ast.Expr
	{
		ident := p.parseIdent()
		// printf is a built-in
		if ident.Name == "printf" {
			fn = &ast.SelectorExpr{
				X:   &ast.Ident{Name: "fmt"},
				Sel: &ast.Ident{Name: "Printf"},
			}
			p.addImportDelc("fmt")
		} else {
			fn = ident
		}
	}

	var args []ast.Expr
	var lparen, rparen scanner.Position
	{
		lparen = p.GoScanner.scnr.Position
		p.expectLit("(")

		for {
			args = append(args, p.parseExpr())
			if p.Curr.Lit == "," {
				p.next()
			} else {
				break
			}
		}

		rparen = p.GoScanner.scnr.Position
		p.expectLit(")")
	}

	return &ast.CallExpr{
		Fun:    fn,
		Lparen: token.Pos(lparen.Offset + 1),
		Rparen: token.Pos(rparen.Offset + 1),
		Args:   args,
	}
}

func (p *Parser) parseSwitchStmt() *ast.SwitchStmt {
	pos := p.expectLit(KeywordSwitch)
	var tag ast.Expr
	{
		exprPos := p.GoScanner.scnr.Position
		str := p.fromHereToEndOfLine()
		tag = p.parseValue(exprPos, str[:len(str)-1])
		p.expectSemicolon()
	}
	body := p.parseSwitchBody()

	return &ast.SwitchStmt{
		Switch: token.Pos(pos.Offset + 1),
		Tag:    tag,
		Body:   body,
	}
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.Curr.Lit {
	case KeywordVar:
		return &ast.DeclStmt{Decl: p.parseVarDecl()}
	case KeywordReturn:
		return p.parseReturnStmt()
	case KeywordSwitch:
		return p.parseSwitchStmt()
	default:
		return &ast.ExprStmt{X: p.parseCallExpr()}
	}
}

func (p *Parser) parseVarDecl() *ast.GenDecl {
	varPos := p.expectLit(KeywordVar)
	varIdent := p.parseIdent()
	var typ ast.Expr
	var val ast.Expr
	if p.Curr.Lit == "=" {
		p.expectLit("=")
		val = p.parseValueToEndOfLine()
	} else {
		// TODO test this
		typ = p.parseType()
		p.expectLit("=")
		val = p.parseValueToEndOfLine()
	}

	return &ast.GenDecl{
		TokPos: token.Pos(varPos.Offset + 1),
		Tok:    token.VAR,
		Specs: []ast.Spec{
			&ast.ValueSpec{
				Names:  []*ast.Ident{varIdent},
				Type:   typ,
				Values: []ast.Expr{val},
			},
		},
	}
}

func (p *Parser) parseStmtsUntil(enders ...string) []ast.Stmt {
	var list []ast.Stmt

FOR:
	for {
		p.skipNewLines()
		for _, end := range enders {
			if p.Curr.Lit == end {
				break FOR
			}
		}
		list = append(list, p.parseStmt())
	}

	return list
}

func (p *Parser) parseBody() *ast.BlockStmt {
	lbrace := p.GoScanner.scnr.Position
	p.expectLit(KeywordBegin)

	list := p.parseStmtsUntil(KeywordEnd)

	rbrace := p.GoScanner.scnr.Position
	p.expectLit(KeywordEnd)

	return &ast.BlockStmt{
		Lbrace: token.Pos(lbrace.Offset + 1),
		List:   list,
		Rbrace: token.Pos(rbrace.Offset + 1),
	}
}

func (p *Parser) parseTypeDecl() *ast.GenDecl {
	typePos := p.expectLit(KeywordType)
	ident := p.parseIdent()
	typ := p.parseType()
	p.expectSemicolon()

	return &ast.GenDecl{
		TokPos: token.Pos(typePos.Offset + 1),
		Tok:    token.TYPE,
		Specs: []ast.Spec{
			&ast.TypeSpec{
				Name: ident,
				Type: typ,
			},
		},
	}
}

func (p *Parser) parseFuncDecl() *ast.FuncDecl {
	funcPos := p.expectLit(KeywordFn)
	var recv *ast.FieldList

	if p.Curr.Lit == "(" {
		// with receiver
		recv = p.parseFieldList()
	}

	ident := p.parseIdent()
	params := p.parseFieldList()
	body := p.parseBody()
	p.expectSemicolon()

	return &ast.FuncDecl{
		Recv: recv,
		Name: ident,
		Type: &ast.FuncType{
			Func:   token.Pos(funcPos.Offset + 1),
			Params: params,
		},
		Body: body,
	}
}

func (p *Parser) parseImportDecl() *ast.GenDecl {
	importPos := p.expectLit(KeywordImport)
	path, pathPos := p.parseString()
	p.expectSemicolon()

	return &ast.GenDecl{
		TokPos: token.Pos(importPos.Offset + 1),
		Tok:    token.IMPORT,
		Specs: []ast.Spec{
			&ast.ImportSpec{
				Path: &ast.BasicLit{
					ValuePos: token.Pos(pathPos.Offset + 1),
					Kind:     token.STRING,
					Value:    path,
				},
			},
		},
	}
}

func (p *Parser) ParseFile() *ast.File {
	p.skipNewLines()
	p.File.Name = &ast.Ident{Name: "main"}

FOR:
	for p.Curr.Tok != scanner.EOF {
		var decl ast.Decl

		switch p.Curr.Lit {
		case KeywordVar:
			decl = p.parseVarDecl()
		case KeywordImport:
			decl = p.parseImportDecl()
		case KeywordFn:
			decl = p.parseFuncDecl()
		case KeywordType:
			decl = p.parseTypeDecl()
		default:
			break FOR
		}
		p.File.Decls = append(p.File.Decls, decl)
	}

	return p.File
}

func (p *Parser) next() {
	if p.Curr.Tok == scanner.EOF {
		return
	}

	p.Curr.Tok = p.GoScanner.scnr.Scan()
	p.Curr.Lit = p.GoScanner.scnr.TokenText()
	if p.verbose {
		p.GoScanner.log(p.Curr.Lit)
	}

	switch p.Curr.Tok {
	case ';', '\n', '\r':
		p.Curr.Tok = scanner.Ident
		p.Curr.Lit = ";"
	}
}

func (p *Parser) skipNewLines() {
	for p.Curr.Lit == KeywordSemicolon {
		p.next()
	}
}

func (p *Parser) expectSemicolon() {
	if p.Curr.Tok == scanner.EOF {
		return
	}
	p.expectLit(KeywordSemicolon)
	p.skipNewLines()
}

func (p *Parser) expectLit(want string) scanner.Position {
	got := p.Curr.Lit
	if got != want {
		panic(fmt.Sprintf("want lit %q; got %q; at %v",
			want, got, p.GoScanner.scnr.Position,
		))
	}
	p.next()
	return p.GoScanner.scnr.Position
}
