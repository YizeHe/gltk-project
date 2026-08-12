// Package lexer tokenizes GrokLang source.
package lexer

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// TokenType classifies tokens.
type TokenType int

const (
	EOF TokenType = iota
	ILLEGAL
	COMMENT

	IDENT
	INT
	FLOAT
	STRING

	// keywords
	FN
	LET
	IF
	ELSE
	WHILE
	FOR
	IN
	RETURN
	IMPORT
	AS
	TRUE
	FALSE
	NULL
	BREAK
	CONTINUE
	SWITCH
	CASE
	DEFAULT
	TRY
	CATCH
	THROW

	// operators
	PLUS
	MINUS
	STAR
	SLASH
	PERCENT
	AMP
	PIPE
	CARET
	TILDE
	BANG
	EQ
	NE
	LT
	LE
	GT
	GE
	ANDAND
	OROR
	ASSIGN
	PLUSPLUS // unused
	QUESTION
	// punct
	LPAREN
	RPAREN
	LBRACE
	RBRACE
	LBRACK
	RBRACK
	COMMA
	COLON
	SEMI
	DOT
	SHL
	SHR
)

// Token is a single lexeme.
type Token struct {
	Type TokenType
	Lit  string
	Line int
	Col  int
}

func (t Token) String() string {
	return fmt.Sprintf("%v(%q)@%d:%d", t.Type, t.Lit, t.Line, t.Col)
}

var keywords = map[string]TokenType{
	"fn":       FN,
	"let":      LET,
	"if":       IF,
	"else":     ELSE,
	"while":    WHILE,
	"for":      FOR,
	"in":       IN,
	"return":   RETURN,
	"import":   IMPORT,
	"as":       AS,
	"true":     TRUE,
	"false":    FALSE,
	"null":     NULL,
	"break":    BREAK,
	"continue": CONTINUE,
	"switch":   SWITCH,
	"case":     CASE,
	"default":  DEFAULT,
	"try":      TRY,
	"catch":    CATCH,
	"throw":    THROW,
}

// Lexer holds scan state.
type Lexer struct {
	src  string
	pos  int
	line int
	col  int
}

// New creates a lexer.
func New(src string) *Lexer {
	return &Lexer{src: src, line: 1, col: 1}
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.pos:])
	return r
}

func (l *Lexer) peek2() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	_, w := utf8.DecodeRuneInString(l.src[l.pos:])
	if l.pos+w >= len(l.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.src[l.pos+w:])
	return r
}

func (l *Lexer) peekN(n int) rune {
	pos := l.pos
	for i := 0; i < n; i++ {
		if pos >= len(l.src) {
			return 0
		}
		_, w := utf8.DecodeRuneInString(l.src[pos:])
		pos += w
	}
	if pos >= len(l.src) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.src[pos:])
	return r
}

func (l *Lexer) next() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	r, w := utf8.DecodeRuneInString(l.src[l.pos:])
	l.pos += w
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

// NextToken returns the next token (skips whitespace and comments as COMMENT skip).
func (l *Lexer) NextToken() Token {
	for {
		l.skipSpace()
		if l.pos >= len(l.src) {
			return Token{Type: EOF, Line: l.line, Col: l.col}
		}
		// line comment //
		if l.peek() == '/' && l.peek2() == '/' {
			l.skipLineComment()
			continue
		}
		// block comment /* */
		if l.peek() == '/' && l.peek2() == '*' {
			if err := l.skipBlockComment(); err != nil {
				return Token{Type: ILLEGAL, Lit: err.Error(), Line: l.line, Col: l.col}
			}
			continue
		}
		break
	}
	line, col := l.line, l.col
	r := l.peek()

	// number
	if unicode.IsDigit(r) {
		return l.lexNumber(line, col)
	}
	// ident / keyword
	if unicode.IsLetter(r) || r == '_' {
		return l.lexIdent(line, col)
	}
	// triple-quoted multiline string """..."""
	if r == '"' && l.peek2() == '"' && l.peekN(2) == '"' {
		return l.lexTripleString(line, col)
	}
	// string
	if r == '"' || r == '\'' {
		return l.lexString(line, col)
	}

	// two-char ops
	switch {
	case r == '=' && l.peek2() == '=':
		l.next()
		l.next()
		return Token{Type: EQ, Lit: "==", Line: line, Col: col}
	case r == '!' && l.peek2() == '=':
		l.next()
		l.next()
		return Token{Type: NE, Lit: "!=", Line: line, Col: col}
	case r == '<' && l.peek2() == '=':
		l.next()
		l.next()
		return Token{Type: LE, Lit: "<=", Line: line, Col: col}
	case r == '>' && l.peek2() == '=':
		l.next()
		l.next()
		return Token{Type: GE, Lit: ">=", Line: line, Col: col}
	case r == '&' && l.peek2() == '&':
		l.next()
		l.next()
		return Token{Type: ANDAND, Lit: "&&", Line: line, Col: col}
	case r == '|' && l.peek2() == '|':
		l.next()
		l.next()
		return Token{Type: OROR, Lit: "||", Line: line, Col: col}
	case r == '<' && l.peek2() == '<':
		l.next()
		l.next()
		return Token{Type: SHL, Lit: "<<", Line: line, Col: col}
	case r == '>' && l.peek2() == '>':
		l.next()
		l.next()
		return Token{Type: SHR, Lit: ">>", Line: line, Col: col}
	}

	l.next()
	switch r {
	case '+':
		return Token{Type: PLUS, Lit: "+", Line: line, Col: col}
	case '-':
		return Token{Type: MINUS, Lit: "-", Line: line, Col: col}
	case '*':
		return Token{Type: STAR, Lit: "*", Line: line, Col: col}
	case '/':
		return Token{Type: SLASH, Lit: "/", Line: line, Col: col}
	case '%':
		return Token{Type: PERCENT, Lit: "%", Line: line, Col: col}
	case '&':
		return Token{Type: AMP, Lit: "&", Line: line, Col: col}
	case '|':
		return Token{Type: PIPE, Lit: "|", Line: line, Col: col}
	case '^':
		return Token{Type: CARET, Lit: "^", Line: line, Col: col}
	case '~':
		return Token{Type: TILDE, Lit: "~", Line: line, Col: col}
	case '!':
		return Token{Type: BANG, Lit: "!", Line: line, Col: col}
	case '<':
		return Token{Type: LT, Lit: "<", Line: line, Col: col}
	case '>':
		return Token{Type: GT, Lit: ">", Line: line, Col: col}
	case '=':
		return Token{Type: ASSIGN, Lit: "=", Line: line, Col: col}
	case '?':
		return Token{Type: QUESTION, Lit: "?", Line: line, Col: col}
	case '(':
		return Token{Type: LPAREN, Lit: "(", Line: line, Col: col}
	case ')':
		return Token{Type: RPAREN, Lit: ")", Line: line, Col: col}
	case '{':
		return Token{Type: LBRACE, Lit: "{", Line: line, Col: col}
	case '}':
		return Token{Type: RBRACE, Lit: "}", Line: line, Col: col}
	case '[':
		return Token{Type: LBRACK, Lit: "[", Line: line, Col: col}
	case ']':
		return Token{Type: RBRACK, Lit: "]", Line: line, Col: col}
	case ',':
		return Token{Type: COMMA, Lit: ",", Line: line, Col: col}
	case ':':
		return Token{Type: COLON, Lit: ":", Line: line, Col: col}
	case ';':
		return Token{Type: SEMI, Lit: ";", Line: line, Col: col}
	case '.':
		return Token{Type: DOT, Lit: ".", Line: line, Col: col}
	default:
		return Token{Type: ILLEGAL, Lit: string(r), Line: line, Col: col}
	}
}

func (l *Lexer) skipSpace() {
	for {
		r := l.peek()
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			l.next()
			continue
		}
		break
	}
}

func (l *Lexer) skipLineComment() {
	for {
		r := l.peek()
		if r == 0 || r == '\n' {
			break
		}
		l.next()
	}
}

func (l *Lexer) skipBlockComment() error {
	l.next() // /
	l.next() // *
	for {
		r := l.peek()
		if r == 0 {
			return fmt.Errorf("unterminated block comment")
		}
		if r == '*' && l.peek2() == '/' {
			l.next()
			l.next()
			return nil
		}
		l.next()
	}
}

func (l *Lexer) lexIdent(line, col int) Token {
	start := l.pos
	for {
		r := l.peek()
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			l.next()
			continue
		}
		break
	}
	lit := l.src[start:l.pos]
	if kw, ok := keywords[lit]; ok {
		return Token{Type: kw, Lit: lit, Line: line, Col: col}
	}
	return Token{Type: IDENT, Lit: lit, Line: line, Col: col}
}

func (l *Lexer) lexNumber(line, col int) Token {
	start := l.pos
	// hex
	if l.peek() == '0' && (l.peek2() == 'x' || l.peek2() == 'X') {
		l.next()
		l.next()
		for {
			r := l.peek()
			if unicode.IsDigit(r) || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
				l.next()
				continue
			}
			break
		}
		return Token{Type: INT, Lit: l.src[start:l.pos], Line: line, Col: col}
	}
	isFloat := false
	for unicode.IsDigit(l.peek()) {
		l.next()
	}
	if l.peek() == '.' && unicode.IsDigit(l.peek2()) {
		isFloat = true
		l.next()
		for unicode.IsDigit(l.peek()) {
			l.next()
		}
	}
	if l.peek() == 'e' || l.peek() == 'E' {
		isFloat = true
		l.next()
		if l.peek() == '+' || l.peek() == '-' {
			l.next()
		}
		for unicode.IsDigit(l.peek()) {
			l.next()
		}
	}
	lit := l.src[start:l.pos]
	if isFloat {
		return Token{Type: FLOAT, Lit: lit, Line: line, Col: col}
	}
	return Token{Type: INT, Lit: lit, Line: line, Col: col}
}

func (l *Lexer) lexString(line, col int) Token {
	quote := l.next()
	var b []rune
	for {
		r := l.peek()
		if r == 0 {
			return Token{Type: ILLEGAL, Lit: "unterminated string", Line: line, Col: col}
		}
		if r == quote {
			l.next()
			break
		}
		// allow newlines only in triple strings; single-line rejects bare newline
		if r == '\n' && quote != 0 {
			// keep newline in content for double-quote (interpolation-friendly multi-line not via """)
			// still allow \n escape; bare newline is illegal for single-quoted, allowed for " for simplicity? reject:
			return Token{Type: ILLEGAL, Lit: "unterminated string", Line: line, Col: col}
		}
		if r == '\\' {
			l.next()
			esc := l.next()
			switch esc {
			case 'n':
				b = append(b, '\n')
			case 't':
				b = append(b, '\t')
			case 'r':
				b = append(b, '\r')
			case '\\', '"', '\'', '$':
				b = append(b, esc)
			case '0':
				b = append(b, 0)
			case 'x':
				// \xHH
				h1, h2 := l.next(), l.next()
				var v byte
				fmt.Sscanf(string([]rune{h1, h2}), "%02x", &v)
				b = append(b, rune(v))
			default:
				b = append(b, esc)
			}
			continue
		}
		// keep ${ as-is for parser interpolation (double-quoted only)
		b = append(b, l.next())
	}
	return Token{Type: STRING, Lit: string(b), Line: line, Col: col}
}

// lexTripleString lexes """ ... """ multiline raw-ish strings (supports escapes and ${}).
func (l *Lexer) lexTripleString(line, col int) Token {
	l.next() // "
	l.next() // "
	l.next() // "
	var b []rune
	for {
		r := l.peek()
		if r == 0 {
			return Token{Type: ILLEGAL, Lit: "unterminated multiline string", Line: line, Col: col}
		}
		if r == '"' && l.peek2() == '"' && l.peekN(2) == '"' {
			l.next()
			l.next()
			l.next()
			break
		}
		if r == '\\' {
			l.next()
			esc := l.next()
			switch esc {
			case 'n':
				b = append(b, '\n')
			case 't':
				b = append(b, '\t')
			case 'r':
				b = append(b, '\r')
			case '\\', '"', '\'', '$':
				b = append(b, esc)
			case '0':
				b = append(b, 0)
			case 'x':
				h1, h2 := l.next(), l.next()
				var v byte
				fmt.Sscanf(string([]rune{h1, h2}), "%02x", &v)
				b = append(b, rune(v))
			default:
				b = append(b, esc)
			}
			continue
		}
		b = append(b, l.next())
	}
	return Token{Type: STRING, Lit: string(b), Line: line, Col: col}
}

// TokenizeAll returns all tokens until EOF.
func TokenizeAll(src string) []Token {
	l := New(src)
	var out []Token
	for {
		t := l.NextToken()
		out = append(out, t)
		if t.Type == EOF || t.Type == ILLEGAL {
			break
		}
	}
	return out
}
