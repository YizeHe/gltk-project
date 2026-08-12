// Package parser builds AST from GrokLang tokens.
package parser

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"groklang/gltk/internal/ast"
	"groklang/gltk/internal/lexer"
)

// Parser is a recursive-descent parser.
type Parser struct {
	lx   *lexer.Lexer
	cur  lexer.Token
	peek lexer.Token
	errs []string
}

// Parse source into a Program.
func Parse(src string) (*ast.Program, error) {
	p := &Parser{lx: lexer.New(src)}
	p.next()
	p.next()
	prog := p.parseProgram()
	if len(p.errs) > 0 {
		return prog, fmt.Errorf("parse: %s", p.errs[0])
	}
	return prog, nil
}

// ParseExpr parses a single expression (for string interpolation).
func ParseExpr(src string) (ast.Expr, error) {
	p := &Parser{lx: lexer.New(src)}
	p.next()
	p.next()
	ex := p.parseExpr()
	if len(p.errs) > 0 {
		return ex, fmt.Errorf("parse: %s", p.errs[0])
	}
	return ex, nil
}

func (p *Parser) next() {
	p.cur = p.peek
	p.peek = p.lx.NextToken()
}

func (p *Parser) expect(tt lexer.TokenType) lexer.Token {
	if p.cur.Type != tt {
		p.errs = append(p.errs, fmt.Sprintf("%d:%d: expected %v got %v (%q)",
			p.cur.Line, p.cur.Col, tt, p.cur.Type, p.cur.Lit))
		return p.cur
	}
	t := p.cur
	p.next()
	return t
}

func (p *Parser) match(tt lexer.TokenType) bool {
	if p.cur.Type == tt {
		p.next()
		return true
	}
	return false
}

func (p *Parser) parseProgram() *ast.Program {
	prog := &ast.Program{Line: 1, Col: 1}
	for p.cur.Type != lexer.EOF && p.cur.Type != lexer.ILLEGAL {
		switch p.cur.Type {
		case lexer.IMPORT:
			prog.Imports = append(prog.Imports, p.parseImport())
		case lexer.FN:
			// top-level: named function only (fn name(...))
			if p.peek.Type == lexer.IDENT {
				prog.Funcs = append(prog.Funcs, p.parseFunc())
			} else {
				// anonymous fn as expression statement (unusual at top-level)
				prog.Stmts = append(prog.Stmts, p.parseStmt())
			}
		default:
			// top-level statement
			prog.Stmts = append(prog.Stmts, p.parseStmt())
		}
	}
	if p.cur.Type == lexer.ILLEGAL {
		p.errs = append(p.errs, fmt.Sprintf("%d:%d: illegal token %q", p.cur.Line, p.cur.Col, p.cur.Lit))
	}
	return prog
}

func (p *Parser) parseImport() *ast.Import {
	t := p.expect(lexer.IMPORT)
	imp := &ast.Import{Line: t.Line, Col: t.Col}
	// Path import: import "path/to/lib.glk" [as alias]
	if p.cur.Type == lexer.STRING {
		imp.Path = p.cur.Lit
		p.next()
		if p.match(lexer.AS) {
			if p.cur.Type != lexer.IDENT {
				p.errs = append(p.errs, "expected alias identifier after as")
			} else {
				imp.Alias = p.cur.Lit
				p.next()
			}
		}
		p.match(lexer.SEMI)
		return imp
	}
	// Bare: import pe, fs, out
	for {
		if p.cur.Type != lexer.IDENT {
			p.errs = append(p.errs, "expected module name or path string")
			break
		}
		imp.Names = append(imp.Names, p.cur.Lit)
		p.next()
		if !p.match(lexer.COMMA) {
			break
		}
	}
	p.match(lexer.SEMI)
	return imp
}

func (p *Parser) parseFunc() *ast.FuncDecl {
	t := p.expect(lexer.FN)
	name := p.expect(lexer.IDENT)
	p.expect(lexer.LPAREN)
	var params []string
	if p.cur.Type != lexer.RPAREN {
		for {
			id := p.expect(lexer.IDENT)
			params = append(params, id.Lit)
			if !p.match(lexer.COMMA) {
				break
			}
		}
	}
	p.expect(lexer.RPAREN)
	body := p.parseBlock()
	return &ast.FuncDecl{Name: name.Lit, Params: params, Body: body, Line: t.Line, Col: t.Col}
}

func (p *Parser) parseFuncExpr() *ast.FuncExpr {
	t := p.expect(lexer.FN)
	p.expect(lexer.LPAREN)
	var params []string
	if p.cur.Type != lexer.RPAREN {
		for {
			id := p.expect(lexer.IDENT)
			params = append(params, id.Lit)
			if !p.match(lexer.COMMA) {
				break
			}
		}
	}
	p.expect(lexer.RPAREN)
	body := p.parseBlock()
	return &ast.FuncExpr{Params: params, Body: body, Line: t.Line, Col: t.Col}
}

func (p *Parser) parseBlock() *ast.BlockStmt {
	t := p.expect(lexer.LBRACE)
	b := &ast.BlockStmt{Line: t.Line, Col: t.Col}
	for p.cur.Type != lexer.RBRACE && p.cur.Type != lexer.EOF {
		b.Stmts = append(b.Stmts, p.parseStmt())
	}
	p.expect(lexer.RBRACE)
	return b
}

func (p *Parser) parseStmt() ast.Stmt {
	switch p.cur.Type {
	case lexer.LET:
		return p.parseLet()
	case lexer.IF:
		return p.parseIf()
	case lexer.WHILE:
		return p.parseWhile()
	case lexer.FOR:
		return p.parseFor()
	case lexer.RETURN:
		return p.parseReturn()
	case lexer.BREAK:
		t := p.cur
		p.next()
		p.match(lexer.SEMI)
		return &ast.BreakStmt{Line: t.Line, Col: t.Col}
	case lexer.CONTINUE:
		t := p.cur
		p.next()
		p.match(lexer.SEMI)
		return &ast.ContinueStmt{Line: t.Line, Col: t.Col}
	case lexer.SWITCH:
		return p.parseSwitch()
	case lexer.TRY:
		return p.parseTry()
	case lexer.THROW:
		return p.parseThrow()
	case lexer.LBRACE:
		return p.parseBlock()
	default:
		// expr or assign
		line, col := p.cur.Line, p.cur.Col
		ex := p.parseExpr()
		if p.cur.Type == lexer.ASSIGN {
			p.next()
			val := p.parseExpr()
			p.match(lexer.SEMI)
			return &ast.AssignStmt{Target: ex, Value: val, Line: line, Col: col}
		}
		p.match(lexer.SEMI)
		return &ast.ExprStmt{X: ex, Line: line, Col: col}
	}
}

func (p *Parser) parseLet() ast.Stmt {
	t := p.expect(lexer.LET)
	name := p.expect(lexer.IDENT)
	p.expect(lexer.ASSIGN)
	val := p.parseExpr()
	p.match(lexer.SEMI)
	return &ast.LetStmt{Name: name.Lit, Value: val, Line: t.Line, Col: t.Col}
}

func (p *Parser) parseIf() ast.Stmt {
	t := p.expect(lexer.IF)
	cond := p.parseExpr()
	then := p.parseBlock()
	var els ast.Stmt
	if p.match(lexer.ELSE) {
		if p.cur.Type == lexer.IF {
			els = p.parseIf()
		} else {
			els = p.parseBlock()
		}
	}
	return &ast.IfStmt{Cond: cond, Then: then, Else: els, Line: t.Line, Col: t.Col}
}

func (p *Parser) parseWhile() ast.Stmt {
	t := p.expect(lexer.WHILE)
	cond := p.parseExpr()
	body := p.parseBlock()
	return &ast.WhileStmt{Cond: cond, Body: body, Line: t.Line, Col: t.Col}
}

func (p *Parser) parseFor() ast.Stmt {
	// for i in expr { }
	t := p.expect(lexer.FOR)
	name := p.expect(lexer.IDENT)
	p.expect(lexer.IN)
	iter := p.parseExpr()
	body := p.parseBlock()
	return &ast.ForInStmt{Name: name.Lit, Iter: iter, Body: body, Line: t.Line, Col: t.Col}
}

func (p *Parser) parseReturn() ast.Stmt {
	t := p.expect(lexer.RETURN)
	var val ast.Expr
	if p.cur.Type != lexer.SEMI && p.cur.Type != lexer.RBRACE {
		val = p.parseExpr()
	}
	p.match(lexer.SEMI)
	return &ast.ReturnStmt{Value: val, Line: t.Line, Col: t.Col}
}

func (p *Parser) parseSwitch() ast.Stmt {
	t := p.expect(lexer.SWITCH)
	tag := p.parseExpr()
	p.expect(lexer.LBRACE)
	sw := &ast.SwitchStmt{Tag: tag, Line: t.Line, Col: t.Col}
	for p.cur.Type != lexer.RBRACE && p.cur.Type != lexer.EOF {
		switch p.cur.Type {
		case lexer.CASE:
			p.next()
			var vals []ast.Expr
			for {
				vals = append(vals, p.parseExpr())
				if !p.match(lexer.COMMA) {
					break
				}
			}
			p.expect(lexer.COLON)
			body := &ast.BlockStmt{Line: p.cur.Line, Col: p.cur.Col}
			for p.cur.Type != lexer.CASE && p.cur.Type != lexer.DEFAULT &&
				p.cur.Type != lexer.RBRACE && p.cur.Type != lexer.EOF {
				body.Stmts = append(body.Stmts, p.parseStmt())
			}
			sw.Cases = append(sw.Cases, ast.SwitchCase{Values: vals, Body: body})
		case lexer.DEFAULT:
			p.next()
			p.expect(lexer.COLON)
			body := &ast.BlockStmt{Line: p.cur.Line, Col: p.cur.Col}
			for p.cur.Type != lexer.CASE && p.cur.Type != lexer.DEFAULT &&
				p.cur.Type != lexer.RBRACE && p.cur.Type != lexer.EOF {
				body.Stmts = append(body.Stmts, p.parseStmt())
			}
			sw.Default = body
		default:
			p.errs = append(p.errs, fmt.Sprintf("%d:%d: expected case or default in switch", p.cur.Line, p.cur.Col))
			p.next()
		}
	}
	p.expect(lexer.RBRACE)
	return sw
}

func (p *Parser) parseTry() ast.Stmt {
	t := p.expect(lexer.TRY)
	body := p.parseBlock()
	p.expect(lexer.CATCH)
	errName := "e"
	if p.cur.Type == lexer.IDENT {
		errName = p.cur.Lit
		p.next()
	}
	catchBody := p.parseBlock()
	return &ast.TryStmt{Body: body, Catch: catchBody, ErrName: errName, Line: t.Line, Col: t.Col}
}

func (p *Parser) parseThrow() ast.Stmt {
	t := p.expect(lexer.THROW)
	val := p.parseExpr()
	p.match(lexer.SEMI)
	return &ast.ThrowStmt{Value: val, Line: t.Line, Col: t.Col}
}

// Pratt-style expression parsing
func (p *Parser) parseExpr() ast.Expr {
	return p.parseTernary()
}

// ternary is lower precedence than || : cond ? a : b
func (p *Parser) parseTernary() ast.Expr {
	cond := p.parseBinary(1)
	if p.cur.Type != lexer.QUESTION {
		return cond
	}
	qt := p.cur
	p.next()
	then := p.parseTernary()
	p.expect(lexer.COLON)
	els := p.parseTernary()
	return &ast.TernaryExpr{Cond: cond, Then: then, Else: els, Line: qt.Line, Col: qt.Col}
}

func precOf(tt lexer.TokenType) int {
	switch tt {
	case lexer.OROR:
		return 1
	case lexer.ANDAND:
		return 2
	case lexer.PIPE:
		return 3
	case lexer.CARET:
		return 4
	case lexer.AMP:
		return 5
	case lexer.EQ, lexer.NE:
		return 6
	case lexer.LT, lexer.LE, lexer.GT, lexer.GE:
		return 7
	case lexer.SHL, lexer.SHR:
		return 8
	case lexer.PLUS, lexer.MINUS:
		return 9
	case lexer.STAR, lexer.SLASH, lexer.PERCENT:
		return 10
	default:
		return 0
	}
}

func opLit(tt lexer.TokenType) string {
	switch tt {
	case lexer.OROR:
		return "||"
	case lexer.ANDAND:
		return "&&"
	case lexer.PIPE:
		return "|"
	case lexer.CARET:
		return "^"
	case lexer.AMP:
		return "&"
	case lexer.EQ:
		return "=="
	case lexer.NE:
		return "!="
	case lexer.LT:
		return "<"
	case lexer.LE:
		return "<="
	case lexer.GT:
		return ">"
	case lexer.GE:
		return ">="
	case lexer.SHL:
		return "<<"
	case lexer.SHR:
		return ">>"
	case lexer.PLUS:
		return "+"
	case lexer.MINUS:
		return "-"
	case lexer.STAR:
		return "*"
	case lexer.SLASH:
		return "/"
	case lexer.PERCENT:
		return "%"
	default:
		return "?"
	}
}

func (p *Parser) parseBinary(minPrec int) ast.Expr {
	left := p.parseUnary()
	for {
		pr := precOf(p.cur.Type)
		if pr < minPrec || pr == 0 {
			break
		}
		opT := p.cur
		p.next()
		right := p.parseBinary(pr + 1)
		left = &ast.BinaryExpr{Op: opLit(opT.Type), Left: left, Right: right, Line: opT.Line, Col: opT.Col}
	}
	return left
}

func (p *Parser) parseUnary() ast.Expr {
	switch p.cur.Type {
	case lexer.BANG, lexer.MINUS, lexer.TILDE:
		op := p.cur
		p.next()
		x := p.parseUnary()
		return &ast.UnaryExpr{Op: op.Lit, X: x, Line: op.Line, Col: op.Col}
	default:
		return p.parsePostfix()
	}
}

func (p *Parser) parsePostfix() ast.Expr {
	ex := p.parsePrimary()
	for {
		switch p.cur.Type {
		case lexer.LPAREN:
			// call
			line, col := p.cur.Line, p.cur.Col
			p.next()
			var args []ast.Expr
			if p.cur.Type != lexer.RPAREN {
				for {
					args = append(args, p.parseExpr())
					if !p.match(lexer.COMMA) {
						break
					}
				}
			}
			p.expect(lexer.RPAREN)
			ex = &ast.CallExpr{Fun: ex, Args: args, Line: line, Col: col}
		case lexer.LBRACK:
			line, col := p.cur.Line, p.cur.Col
			p.next()
			idx := p.parseExpr()
			p.expect(lexer.RBRACK)
			ex = &ast.IndexExpr{X: ex, Index: idx, Line: line, Col: col}
		case lexer.DOT:
			line, col := p.cur.Line, p.cur.Col
			p.next()
			name := p.expect(lexer.IDENT)
			ex = &ast.FieldExpr{X: ex, Name: name.Lit, Line: line, Col: col}
		default:
			return ex
		}
	}
}

func (p *Parser) parsePrimary() ast.Expr {
	t := p.cur
	switch t.Type {
	case lexer.IDENT:
		p.next()
		return &ast.Ident{Name: t.Lit, Line: t.Line, Col: t.Col}
	case lexer.INT:
		p.next()
		var v int64
		if len(t.Lit) > 2 && (t.Lit[1] == 'x' || t.Lit[1] == 'X') {
			v, _ = strconv.ParseInt(t.Lit[2:], 16, 64)
		} else {
			v, _ = strconv.ParseInt(t.Lit, 10, 64)
		}
		return &ast.Literal{Kind: ast.LitInt, Int: v, Line: t.Line, Col: t.Col}
	case lexer.FLOAT:
		p.next()
		f, _ := strconv.ParseFloat(t.Lit, 64)
		return &ast.Literal{Kind: ast.LitFloat, Float: f, Line: t.Line, Col: t.Col}
	case lexer.STRING:
		p.next()
		if strings.Contains(t.Lit, "${") {
			return p.buildInterp(t.Lit, t.Line, t.Col)
		}
		return &ast.Literal{Kind: ast.LitStr, Str: t.Lit, Line: t.Line, Col: t.Col}
	case lexer.TRUE:
		p.next()
		return &ast.Literal{Kind: ast.LitBool, Bool: true, Line: t.Line, Col: t.Col}
	case lexer.FALSE:
		p.next()
		return &ast.Literal{Kind: ast.LitBool, Bool: false, Line: t.Line, Col: t.Col}
	case lexer.NULL:
		p.next()
		return &ast.Literal{Kind: ast.LitNull, Line: t.Line, Col: t.Col}
	case lexer.FN:
		// anonymous function expression
		return p.parseFuncExpr()
	case lexer.LPAREN:
		p.next()
		ex := p.parseExpr()
		p.expect(lexer.RPAREN)
		return ex
	case lexer.LBRACK:
		return p.parseArray()
	case lexer.LBRACE:
		return p.parseMap()
	default:
		p.errs = append(p.errs, fmt.Sprintf("%d:%d: unexpected token %q", t.Line, t.Col, t.Lit))
		p.next()
		return &ast.Literal{Kind: ast.LitNull, Line: t.Line, Col: t.Col}
	}
}

// buildInterp turns "a${x+1}b" into BinaryExpr concat tree: "a" + (x+1) + "b"
func (p *Parser) buildInterp(s string, line, col int) ast.Expr {
	var parts []ast.Expr
	i := 0
	for i < len(s) {
		// find next ${
		j := strings.Index(s[i:], "${")
		if j < 0 {
			if i < len(s) {
				parts = append(parts, &ast.Literal{Kind: ast.LitStr, Str: s[i:], Line: line, Col: col})
			}
			break
		}
		j += i
		if j > i {
			parts = append(parts, &ast.Literal{Kind: ast.LitStr, Str: s[i:j], Line: line, Col: col})
		}
		// parse expr until matching }
		k := j + 2
		depth := 1
		for k < len(s) && depth > 0 {
			r, w := utf8.DecodeRuneInString(s[k:])
			if r == '{' {
				depth++
			} else if r == '}' {
				depth--
				if depth == 0 {
					break
				}
			}
			k += w
		}
		if depth != 0 || k >= len(s) {
			p.errs = append(p.errs, fmt.Sprintf("%d:%d: unclosed ${ in string interpolation", line, col))
			parts = append(parts, &ast.Literal{Kind: ast.LitStr, Str: s[j:], Line: line, Col: col})
			break
		}
		inner := s[j+2 : k]
		ex, err := ParseExpr(inner)
		if err != nil {
			p.errs = append(p.errs, fmt.Sprintf("%d:%d: bad interpolation expr: %v", line, col, err))
			ex = &ast.Literal{Kind: ast.LitStr, Str: "", Line: line, Col: col}
		}
		parts = append(parts, ex)
		i = k + 1 // skip }
	}
	if len(parts) == 0 {
		return &ast.Literal{Kind: ast.LitStr, Str: "", Line: line, Col: col}
	}
	// fold with +
	acc := parts[0]
	for i := 1; i < len(parts); i++ {
		acc = &ast.BinaryExpr{Op: "+", Left: acc, Right: parts[i], Line: line, Col: col}
	}
	return acc
}

func (p *Parser) parseArray() ast.Expr {
	t := p.expect(lexer.LBRACK)
	arr := &ast.ArrayExpr{Line: t.Line, Col: t.Col}
	if p.cur.Type != lexer.RBRACK {
		for {
			arr.Elts = append(arr.Elts, p.parseExpr())
			if !p.match(lexer.COMMA) {
				break
			}
			if p.cur.Type == lexer.RBRACK {
				break
			}
		}
	}
	p.expect(lexer.RBRACK)
	return arr
}

func (p *Parser) parseMap() ast.Expr {
	// { key: val, ... }  key is IDENT or STRING
	t := p.expect(lexer.LBRACE)
	m := &ast.MapExpr{Line: t.Line, Col: t.Col}
	if p.cur.Type != lexer.RBRACE {
		for {
			var key ast.Expr
			if p.cur.Type == lexer.IDENT {
				key = &ast.Literal{Kind: ast.LitStr, Str: p.cur.Lit, Line: p.cur.Line, Col: p.cur.Col}
				p.next()
			} else if p.cur.Type == lexer.STRING {
				key = &ast.Literal{Kind: ast.LitStr, Str: p.cur.Lit, Line: p.cur.Line, Col: p.cur.Col}
				p.next()
			} else {
				key = p.parseExpr()
			}
			p.expect(lexer.COLON)
			val := p.parseExpr()
			m.Keys = append(m.Keys, key)
			m.Vals = append(m.Vals, val)
			if !p.match(lexer.COMMA) {
				break
			}
			if p.cur.Type == lexer.RBRACE {
				break
			}
		}
	}
	p.expect(lexer.RBRACE)
	return m
}
