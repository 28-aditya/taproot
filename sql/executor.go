package sql

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"taproot/storage"
)

// Result is the shape every statement returns: SELECT/DESCRIBE/SHOW
// populate Columns+Rows, INSERT/UPDATE/DELETE populate RowsAffected, and
// everything sets a human-readable Message.
type Result struct {
	Columns      []string
	Rows         [][]any
	RowsAffected int
	Message      string
}

func (r *Result) String() string {
	if len(r.Columns) == 0 {
		return r.Message
	}

	var sb strings.Builder
	sb.WriteString(strings.Join(r.Columns, " | "))
	sb.WriteByte('\n')
	for _, row := range r.Rows {
		parts := make([]string, len(row))
		for i, v := range row {
			parts[i] = fmt.Sprintf("%v", v)
		}
		sb.WriteString(strings.Join(parts, " | "))
		sb.WriteByte('\n')
	}
	if r.Message != "" {
		sb.WriteString(r.Message)
	}
	return sb.String()
}

// Run tokenizes, parses, and executes a single SQL statement against cat.
func Run(query string, cat *Catalog) (*Result, error) {
	stmt, err := Parse(query)
	if err != nil {
		return nil, err
	}
	return Execute(stmt, cat)
}

// Execute runs an already-parsed statement against cat.
func Execute(stmt Statement, cat *Catalog) (*Result, error) {
	switch s := stmt.(type) {
	case *CreateTableStmt:
		return execCreateTable(s, cat)
	case *DropTableStmt:
		return execDropTable(s, cat)
	case *DescribeStmt:
		return execDescribe(s, cat)
	case *ShowTablesStmt:
		return execShowTables(s, cat)
	case *InsertStmt:
		return execInsert(s, cat)
	case *SelectStmt:
		return execSelect(s, cat)
	case *UpdateStmt:
		return execUpdate(s, cat)
	case *DeleteStmt:
		return execDelete(s, cat)
	default:
		return nil, fmt.Errorf("unsupported statement type %T", stmt)
	}
}

func execCreateTable(s *CreateTableStmt, cat *Catalog) (*Result, error) {
	columns := make([]Column, len(s.Columns))
	for i, c := range s.Columns {
		columns[i] = Column{Name: c.Name, Type: c.Type, PrimaryKey: c.PrimaryKey}
	}

	schema, err := cat.CreateTable(s.Table, columns)
	if err != nil {
		return nil, err
	}
	return &Result{Message: fmt.Sprintf("table %q created", schema.Name)}, nil
}

func execDropTable(s *DropTableStmt, cat *Catalog) (*Result, error) {
	if err := cat.DropTable(s.Table); err != nil {
		return nil, err
	}
	return &Result{Message: fmt.Sprintf("table %q dropped", s.Table)}, nil
}

func execDescribe(s *DescribeStmt, cat *Catalog) (*Result, error) {
	schema, err := cat.GetSchema(s.Table)
	if err != nil {
		return nil, err
	}

	rows := make([][]any, len(schema.Columns))
	for i, c := range schema.Columns {
		key := ""
		if c.PrimaryKey {
			key = "PRIMARY KEY"
		}
		rows[i] = []any{c.Name, c.Type.String(), key}
	}

	return &Result{Columns: []string{"column", "type", "key"}, Rows: rows}, nil
}

func execShowTables(s *ShowTablesStmt, cat *Catalog) (*Result, error) {
	names := cat.TableNames()
	rows := make([][]any, len(names))
	for i, n := range names {
		rows[i] = []any{n}
	}
	return &Result{Columns: []string{"table"}, Rows: rows}, nil
}

func execInsert(s *InsertStmt, cat *Catalog) (*Result, error) {
	schema, err := cat.GetSchema(s.Table)
	if err != nil {
		return nil, err
	}
	tree, err := cat.GetTree(s.Table)
	if err != nil {
		return nil, err
	}

	columns := s.Columns
	if len(columns) == 0 {
		columns = make([]string, len(schema.Columns))
		for i, c := range schema.Columns {
			columns[i] = c.Name
		}
	}
	if len(columns) != len(s.Values) {
		return nil, fmt.Errorf("column count (%d) doesn't match value count (%d)", len(columns), len(s.Values))
	}

	row := make(map[string]any, len(schema.Columns))
	for i, colName := range columns {
		col, ok := schema.Column(colName)
		if !ok {
			return nil, fmt.Errorf("table %q has no column %q", s.Table, colName)
		}
		val, err := evalLiteral(s.Values[i])
		if err != nil {
			return nil, err
		}
		coerced, err := coerceValue(val, col.Type)
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", colName, err)
		}
		row[col.Name] = coerced
	}
	for _, c := range schema.Columns {
		if _, ok := row[c.Name]; !ok {
			row[c.Name] = nil
		}
	}

	pkCol := schema.PrimaryKey
	pkValue, hasPK := row[pkCol].(int)
	if !hasPK {
		schema.NextRowID++
		pkValue = schema.NextRowID
		row[pkCol] = pkValue
	} else {
		if pkValue >= schema.NextRowID {
			schema.NextRowID = pkValue + 1
		}
		if _, found := tree.SearchTree(pkValue); found {
			return nil, fmt.Errorf("duplicate value %d for primary key %q", pkValue, pkCol)
		}
	}

	if err := tree.Insert(pkValue, row); err != nil {
		return nil, err
	}
	if err := cat.Flush(); err != nil {
		return nil, err
	}

	return &Result{RowsAffected: 1, Message: "1 row inserted"}, nil
}

func execSelect(s *SelectStmt, cat *Catalog) (*Result, error) {
	schema, err := cat.GetSchema(s.Table)
	if err != nil {
		return nil, err
	}
	tree, err := cat.GetTree(s.Table)
	if err != nil {
		return nil, err
	}

	columns := s.Columns
	if len(columns) == 0 {
		columns = make([]string, len(schema.Columns))
		for i, c := range schema.Columns {
			columns[i] = c.Name
		}
	}
	for _, colName := range columns {
		if _, ok := schema.Column(colName); !ok {
			return nil, fmt.Errorf("table %q has no column %q", s.Table, colName)
		}
	}

	matches, err := scanTable(tree, s.Where)
	if err != nil {
		return nil, err
	}

	rows := make([][]any, 0, len(matches))
	for _, row := range matches {
		outRow := make([]any, len(columns))
		for i, colName := range columns {
			outRow[i] = row[colName]
		}
		rows = append(rows, outRow)
	}

	return &Result{Columns: columns, Rows: rows}, nil
}

func execUpdate(s *UpdateStmt, cat *Catalog) (*Result, error) {
	schema, err := cat.GetSchema(s.Table)
	if err != nil {
		return nil, err
	}
	tree, err := cat.GetTree(s.Table)
	if err != nil {
		return nil, err
	}
	for _, set := range s.Set {
		if _, ok := schema.Column(set.Column); !ok {
			return nil, fmt.Errorf("table %q has no column %q", s.Table, set.Column)
		}
	}

	matches, err := scanTable(tree, s.Where)
	if err != nil {
		return nil, err
	}

	affected := 0
	for _, row := range matches {
		oldPK, ok := row[schema.PrimaryKey].(int)
		if !ok {
			continue
		}

		updated := make(map[string]any, len(row))
		for k, v := range row {
			updated[k] = v
		}
		for _, set := range s.Set {
			val, err := evalExpr(set.Value, row)
			if err != nil {
				return nil, err
			}
			col, _ := schema.Column(set.Column)
			coerced, err := coerceValue(val, col.Type)
			if err != nil {
				return nil, fmt.Errorf("column %q: %w", set.Column, err)
			}
			updated[col.Name] = coerced
		}

		newPK, _ := updated[schema.PrimaryKey].(int)
		if newPK != oldPK {
			if _, found := tree.SearchTree(newPK); found {
				return nil, fmt.Errorf("update would duplicate primary key value %d", newPK)
			}
		}
		// bPlusTree.Insert doesn't overwrite an existing key in place, it
		// just inserts another entry for it — delete the old key first so
		// an update (even one that doesn't touch the PK) doesn't leave a
		// stale duplicate behind.
		tree.DeleteLeaf(oldPK)
		if err := tree.Insert(newPK, updated); err != nil {
			return nil, err
		}
		affected++
	}

	if err := cat.Flush(); err != nil {
		return nil, err
	}

	return &Result{RowsAffected: affected, Message: fmt.Sprintf("%d row(s) updated", affected)}, nil
}

func execDelete(s *DeleteStmt, cat *Catalog) (*Result, error) {
	schema, err := cat.GetSchema(s.Table)
	if err != nil {
		return nil, err
	}
	tree, err := cat.GetTree(s.Table)
	if err != nil {
		return nil, err
	}

	matches, err := scanTable(tree, s.Where)
	if err != nil {
		return nil, err
	}

	affected := 0
	for _, row := range matches {
		pkVal, ok := row[schema.PrimaryKey].(int)
		if !ok {
			continue
		}
		if tree.DeleteLeaf(pkVal) {
			affected++
		}
	}

	if err := cat.Flush(); err != nil {
		return nil, err
	}

	return &Result{RowsAffected: affected, Message: fmt.Sprintf("%d row(s) deleted", affected)}, nil
}

// --- scanning & expression evaluation ---

// scanTable walks every row in tree (there's no per-index lookup below
// the primary key yet, so WHERE is evaluated by a full scan) and returns
// the ones matching where, in primary-key order. where == nil returns
// every row.
func scanTable(tree *storage.Tree, where Expr) ([]map[string]any, error) {
	all := tree.RangeScan(math.MinInt, math.MaxInt)

	keys := make([]int, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	var matches []map[string]any
	for _, k := range keys {
		row, ok := all[k].(map[string]any)
		if !ok {
			continue
		}
		if where == nil {
			matches = append(matches, row)
			continue
		}

		result, err := evalExpr(where, row)
		if err != nil {
			return nil, err
		}
		if b, ok := result.(bool); ok && b {
			matches = append(matches, row)
		}
	}

	return matches, nil
}

func evalLiteral(e Expr) (any, error) {
	lit, ok := e.(*Literal)
	if !ok {
		return nil, fmt.Errorf("expected a literal value, got %T", e)
	}
	return lit.Value, nil
}

func coerceValue(val any, want ColumnType) (any, error) {
	if val == nil {
		return nil, nil
	}
	switch want {
	case TypeInt:
		if v, ok := val.(int); ok {
			return v, nil
		}
		return nil, fmt.Errorf("expected INT, got %T", val)
	case TypeFloat:
		switch v := val.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		}
		return nil, fmt.Errorf("expected FLOAT, got %T", val)
	case TypeText:
		if v, ok := val.(string); ok {
			return v, nil
		}
		return nil, fmt.Errorf("expected TEXT, got %T", val)
	case TypeBool:
		if v, ok := val.(bool); ok {
			return v, nil
		}
		return nil, fmt.Errorf("expected BOOL, got %T", val)
	default:
		return val, nil
	}
}

// evalExpr evaluates expr against a single row for WHERE/SET clauses.
func evalExpr(e Expr, row map[string]any) (any, error) {
	switch expr := e.(type) {
	case *Literal:
		return expr.Value, nil

	case *ColumnRef:
		val, ok := row[expr.Name]
		if !ok {
			return nil, fmt.Errorf("no such column %q", expr.Name)
		}
		return val, nil

	case *BinaryExpr:
		switch expr.Op {
		case "AND":
			left, err := evalExpr(expr.Left, row)
			if err != nil {
				return nil, err
			}
			if lb, ok := left.(bool); !ok || !lb {
				return false, nil
			}
			right, err := evalExpr(expr.Right, row)
			if err != nil {
				return nil, err
			}
			rb, _ := right.(bool)
			return rb, nil

		case "OR":
			left, err := evalExpr(expr.Left, row)
			if err != nil {
				return nil, err
			}
			if lb, ok := left.(bool); ok && lb {
				return true, nil
			}
			right, err := evalExpr(expr.Right, row)
			if err != nil {
				return nil, err
			}
			rb, _ := right.(bool)
			return rb, nil

		default:
			left, err := evalExpr(expr.Left, row)
			if err != nil {
				return nil, err
			}
			right, err := evalExpr(expr.Right, row)
			if err != nil {
				return nil, err
			}
			return compare(left, expr.Op, right)
		}

	default:
		return nil, fmt.Errorf("cannot evaluate expression of type %T", e)
	}
}

func compare(left any, op string, right any) (bool, error) {
	if lf, lok := toFloat(left); lok {
		if rf, rok := toFloat(right); rok {
			switch op {
			case "=":
				return lf == rf, nil
			case "!=", "<>":
				return lf != rf, nil
			case "<":
				return lf < rf, nil
			case "<=":
				return lf <= rf, nil
			case ">":
				return lf > rf, nil
			case ">=":
				return lf >= rf, nil
			default:
				return false, fmt.Errorf("unsupported operator %q", op)
			}
		}
	}

	if ls, lok := left.(string); lok {
		if rs, rok := right.(string); rok {
			switch op {
			case "=":
				return ls == rs, nil
			case "!=", "<>":
				return ls != rs, nil
			case "<":
				return ls < rs, nil
			case "<=":
				return ls <= rs, nil
			case ">":
				return ls > rs, nil
			case ">=":
				return ls >= rs, nil
			default:
				return false, fmt.Errorf("unsupported operator %q", op)
			}
		}
	}

	if lb, lok := left.(bool); lok {
		if rb, rok := right.(bool); rok {
			switch op {
			case "=":
				return lb == rb, nil
			case "!=", "<>":
				return lb != rb, nil
			default:
				return false, fmt.Errorf("operator %q not supported for BOOL", op)
			}
		}
	}

	if left == nil || right == nil {
		switch op {
		case "=":
			return left == nil && right == nil, nil
		case "!=", "<>":
			return !(left == nil && right == nil), nil
		default:
			return false, nil
		}
	}

	return false, fmt.Errorf("cannot compare %T and %T", left, right)
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}