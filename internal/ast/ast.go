// Package ast defines GrokLang abstract syntax tree nodes.
package ast

// Node is any AST node.
type Node interface {
	Pos() (line, col int)
}

// Program is a full source file.
type Program struct {
	Imports []*Import
	Funcs   []*FuncDecl
	// top-level statements (optional, executed before main if any)
	Stmts []Stmt
	Line  int
	Col   int
}

func (p *Program) Pos() (int, int) { return p.Line, p.Col }

// Import is either bare native names or a path library import.
// Bare: import pe, fs, out  (Path empty, Names filled)
// Path: import "libs/helpers.glk" [as alias]
type Import struct {
	Names []string // bare names (pe, fs) — when Path empty
	Path  string   // string path for .glk library
	Alias string   // optional alias for path import (default = filename stem)
	Line  int
	Col   int
}

func (i *Import) Pos() (int, int) { return i.Line, i.Col }

// FuncDecl: fn name(params) { body }
type FuncDecl struct {
	Name   string
	Params []string
	Body   *BlockStmt
	Line   int
	Col    int
}

func (f *FuncDecl) Pos() (int, int) { return f.Line, f.Col }

// Stmt nodes
type Stmt interface {
	Node
	stmtNode()
}

type BlockStmt struct {
	Stmts []Stmt
	Line  int
	Col   int
}

func (b *BlockStmt) Pos() (int, int) { return b.Line, b.Col }
func (b *BlockStmt) stmtNode()       {}

type LetStmt struct {
	Name  string
	Value Expr
	Line  int
	Col   int
}

func (s *LetStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *LetStmt) stmtNode()       {}

type AssignStmt struct {
	Target Expr // Ident, Index, Field
	Value  Expr
	Line   int
	Col    int
}

func (s *AssignStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *AssignStmt) stmtNode()       {}

type ExprStmt struct {
	X    Expr
	Line int
	Col  int
}

func (s *ExprStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *ExprStmt) stmtNode()       {}

type ReturnStmt struct {
	Value Expr // may be nil
	Line  int
	Col   int
}

func (s *ReturnStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *ReturnStmt) stmtNode()       {}

type IfStmt struct {
	Cond Expr
	Then *BlockStmt
	Else Stmt // BlockStmt or IfStmt or nil
	Line int
	Col  int
}

func (s *IfStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *IfStmt) stmtNode()       {}

type WhileStmt struct {
	Cond Expr
	Body *BlockStmt
	Line int
	Col  int
}

func (s *WhileStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *WhileStmt) stmtNode()       {}

type ForInStmt struct {
	Name string
	Iter Expr
	Body *BlockStmt
	Line int
	Col  int
}

func (s *ForInStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *ForInStmt) stmtNode()       {}

// BreakStmt: break;
type BreakStmt struct {
	Line int
	Col  int
}

func (s *BreakStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *BreakStmt) stmtNode()       {}

// ContinueStmt: continue;
type ContinueStmt struct {
	Line int
	Col  int
}

func (s *ContinueStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *ContinueStmt) stmtNode()       {}

// SwitchCase is one case arm: case a, b: stmts
type SwitchCase struct {
	Values []Expr
	Body   *BlockStmt
}

// SwitchStmt: switch expr { case ... default ... }
type SwitchStmt struct {
	Tag     Expr
	Cases   []SwitchCase
	Default *BlockStmt // optional
	Line    int
	Col     int
}

func (s *SwitchStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *SwitchStmt) stmtNode()       {}

// TryStmt: try { ... } catch e { ... }
type TryStmt struct {
	Body    *BlockStmt
	Catch   *BlockStmt
	ErrName string // catch binding
	Line    int
	Col     int
}

func (s *TryStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *TryStmt) stmtNode()       {}

// ThrowStmt: throw expr;
type ThrowStmt struct {
	Value Expr
	Line  int
	Col   int
}

func (s *ThrowStmt) Pos() (int, int) { return s.Line, s.Col }
func (s *ThrowStmt) stmtNode()       {}

// Expr nodes
type Expr interface {
	Node
	exprNode()
}

type Ident struct {
	Name string
	Line int
	Col  int
}

func (e *Ident) Pos() (int, int) { return e.Line, e.Col }
func (e *Ident) exprNode()       {}

type Literal struct {
	Kind  LitKind
	Int   int64
	Float float64
	Str   string
	Bool  bool
	Line  int
	Col   int
}

type LitKind int

const (
	LitNull LitKind = iota
	LitBool
	LitInt
	LitFloat
	LitStr
)

func (e *Literal) Pos() (int, int) { return e.Line, e.Col }
func (e *Literal) exprNode()       {}

type BinaryExpr struct {
	Op    string
	Left  Expr
	Right Expr
	Line  int
	Col   int
}

func (e *BinaryExpr) Pos() (int, int) { return e.Line, e.Col }
func (e *BinaryExpr) exprNode()       {}

type UnaryExpr struct {
	Op   string
	X    Expr
	Line int
	Col  int
}

func (e *UnaryExpr) Pos() (int, int) { return e.Line, e.Col }
func (e *UnaryExpr) exprNode()       {}

// TernaryExpr: cond ? then : else
type TernaryExpr struct {
	Cond Expr
	Then Expr
	Else Expr
	Line int
	Col  int
}

func (e *TernaryExpr) Pos() (int, int) { return e.Line, e.Col }
func (e *TernaryExpr) exprNode()       {}

// FuncExpr: anonymous function / lambda: fn(params) { body }
type FuncExpr struct {
	Params []string
	Body   *BlockStmt
	Line   int
	Col    int
}

func (e *FuncExpr) Pos() (int, int) { return e.Line, e.Col }
func (e *FuncExpr) exprNode()       {}

type CallExpr struct {
	Fun  Expr
	Args []Expr
	Line int
	Col  int
}

func (e *CallExpr) Pos() (int, int) { return e.Line, e.Col }
func (e *CallExpr) exprNode()       {}

type IndexExpr struct {
	X     Expr
	Index Expr
	Line  int
	Col   int
}

func (e *IndexExpr) Pos() (int, int) { return e.Line, e.Col }
func (e *IndexExpr) exprNode()       {}

type FieldExpr struct {
	X    Expr
	Name string
	Line int
	Col  int
}

func (e *FieldExpr) Pos() (int, int) { return e.Line, e.Col }
func (e *FieldExpr) exprNode()       {}

type ArrayExpr struct {
	Elts []Expr
	Line int
	Col  int
}

func (e *ArrayExpr) Pos() (int, int) { return e.Line, e.Col }
func (e *ArrayExpr) exprNode()       {}

type MapExpr struct {
	Keys []Expr // string literals or idents
	Vals []Expr
	Line int
	Col  int
}

func (e *MapExpr) Pos() (int, int) { return e.Line, e.Col }
func (e *MapExpr) exprNode()       {}
