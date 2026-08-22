package sql

// Statement is implemented by every parsed SQL statement type.
type Statement interface{ stmtNode() }

type ColumnDef struct {
	Name       string
	Type       ColumnType
	PrimaryKey bool
}

type CreateTableStmt struct {
	Table   string
	Columns []ColumnDef
}

type DropTableStmt struct {
	Table string
}

type DescribeStmt struct {
	Table string
}

type ShowTablesStmt struct{}

type InsertStmt struct {
	Table   string
	Columns []string // nil = use every schema column, in schema order
	Values  []Expr
}

type SelectStmt struct {
	Table   string
	Columns []string // nil/empty = SELECT *
	Where   Expr     // nil = no WHERE clause
}

type SetClause struct {
	Column string
	Value  Expr
}

type UpdateStmt struct {
	Table string
	Set   []SetClause
	Where Expr
}

type DeleteStmt struct {
	Table string
	Where Expr
}

func (*CreateTableStmt) stmtNode() {}
func (*DropTableStmt) stmtNode()   {}
func (*DescribeStmt) stmtNode()    {}
func (*ShowTablesStmt) stmtNode()  {}
func (*InsertStmt) stmtNode()      {}
func (*SelectStmt) stmtNode()      {}
func (*UpdateStmt) stmtNode()      {}
func (*DeleteStmt) stmtNode()      {}

// Expr is implemented by every parsed expression node (used in WHERE and
// SET clauses, and for VALUES literals).
type Expr interface{ exprNode() }

// Literal is a constant value: int, float64, string, bool, or nil (SQL NULL).
type Literal struct {
	Value any
}

// ColumnRef refers to a column by name, resolved against a row at eval time.
type ColumnRef struct {
	Name string
}

// BinaryExpr covers comparisons (=, !=, <>, <, <=, >, >=) and boolean
// combinators (AND, OR).
type BinaryExpr struct {
	Left  Expr
	Op    string
	Right Expr
}

func (*Literal) exprNode()    {}
func (*ColumnRef) exprNode()  {}
func (*BinaryExpr) exprNode() {}