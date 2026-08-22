package sql

import (
	"fmt"
	"strconv"
)

type Parser struct {
	tokens []Token
	pos    int
}

// Parse tokenizes and parses a single SQL statement.
func Parse(input string) (Statement, error) {
	tokens, err := Tokenize(input)
	if err != nil {
		return nil, err
	}

	p := &Parser{tokens: tokens}
	stmt, err := p.parseStatement()
	if err != nil {
		return nil, err
	}

	if p.check(TokenSemicolon) {
		p.advance()
	}
	if !p.check(TokenEOF) {
		return nil, fmt.Errorf("unexpected token %q after statement", p.peek().Literal)
	}

	return stmt, nil
}

// --- token stream helpers ---

func (p *Parser) peek() Token {
	return p.tokens[p.pos]
}

func (p *Parser) advance() Token {
	t := p.tokens[p.pos]
	if p.pos < len(p.tokens)-1 {
		p.pos++
	}
	return t
}

func (p *Parser) check(t TokenType) bool {
	return p.peek().Type == t
}

func (p *Parser) checkKeyword(kw string) bool {
	tok := p.peek()
	return tok.Type == TokenKeyword && tok.Literal == kw
}

func (p *Parser) expect(t TokenType, what string) (Token, error) {
	if !p.check(t) {
		return Token{}, fmt.Errorf("expected %s, got %q", what, p.peek().Literal)
	}
	return p.advance(), nil
}

func (p *Parser) expectKeyword(kw string) error {
	if !p.checkKeyword(kw) {
		return fmt.Errorf("expected keyword %s, got %q", kw, p.peek().Literal)
	}
	p.advance()
	return nil
}

// --- statements ---

func (p *Parser) parseStatement() (Statement, error) {
	tok := p.peek()
	if tok.Type != TokenKeyword {
		return nil, fmt.Errorf("expected a statement keyword, got %q", tok.Literal)
	}

	switch tok.Literal {
	case "SELECT":
		return p.parseSelect()
	case "INSERT":
		return p.parseInsert()
	case "UPDATE":
		return p.parseUpdate()
	case "DELETE":
		return p.parseDelete()
	case "CREATE":
		return p.parseCreateTable()
	case "DROP":
		return p.parseDropTable()
	case "DESC", "DESCRIBE":
		return p.parseDescribe()
	case "SHOW":
		return p.parseShowTables()
	default:
		return nil, fmt.Errorf("unsupported statement %q", tok.Literal)
	}
}

// CREATE TABLE name ( col type [PRIMARY KEY], ... )
func (p *Parser) parseCreateTable() (Statement, error) {
	if err := p.expectKeyword("CREATE"); err != nil {
		return nil, err
	}
	if err := p.expectKeyword("TABLE"); err != nil {
		return nil, err
	}
	nameTok, err := p.expect(TokenIdent, "table name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenLParen, "'('"); err != nil {
		return nil, err
	}

	var columns []ColumnDef
	for {
		colNameTok, err := p.expect(TokenIdent, "column name")
		if err != nil {
			return nil, err
		}

		typeTok := p.advance()
		if typeTok.Type != TokenIdent && typeTok.Type != TokenKeyword {
			return nil, fmt.Errorf("expected column type, got %q", typeTok.Literal)
		}
		colType, err := ParseColumnType(typeTok.Literal)
		if err != nil {
			return nil, err
		}

		primaryKey := false
		if p.checkKeyword("PRIMARY") {
			p.advance()
			if err := p.expectKeyword("KEY"); err != nil {
				return nil, err
			}
			primaryKey = true
		}

		columns = append(columns, ColumnDef{Name: colNameTok.Literal, Type: colType, PrimaryKey: primaryKey})

		if p.check(TokenComma) {
			p.advance()
			continue
		}
		break
	}

	if _, err := p.expect(TokenRParen, "')'"); err != nil {
		return nil, err
	}

	return &CreateTableStmt{Table: nameTok.Literal, Columns: columns}, nil
}

// DROP TABLE name
func (p *Parser) parseDropTable() (Statement, error) {
	if err := p.expectKeyword("DROP"); err != nil {
		return nil, err
	}
	if err := p.expectKeyword("TABLE"); err != nil {
		return nil, err
	}
	nameTok, err := p.expect(TokenIdent, "table name")
	if err != nil {
		return nil, err
	}
	return &DropTableStmt{Table: nameTok.Literal}, nil
}

// DESC name | DESCRIBE name
func (p *Parser) parseDescribe() (Statement, error) {
	p.advance() // DESC or DESCRIBE
	nameTok, err := p.expect(TokenIdent, "table name")
	if err != nil {
		return nil, err
	}
	return &DescribeStmt{Table: nameTok.Literal}, nil
}

// SHOW TABLES
func (p *Parser) parseShowTables() (Statement, error) {
	if err := p.expectKeyword("SHOW"); err != nil {
		return nil, err
	}
	if err := p.expectKeyword("TABLES"); err != nil {
		return nil, err
	}
	return &ShowTablesStmt{}, nil
}

// SELECT (* | col, col, ...) FROM name [WHERE expr]
func (p *Parser) parseSelect() (Statement, error) {
	if err := p.expectKeyword("SELECT"); err != nil {
		return nil, err
	}

	var columns []string
	if p.check(TokenStar) {
		p.advance()
	} else {
		for {
			colTok, err := p.expect(TokenIdent, "column name")
			if err != nil {
				return nil, err
			}
			columns = append(columns, colTok.Literal)
			if p.check(TokenComma) {
				p.advance()
				continue
			}
			break
		}
	}

	if err := p.expectKeyword("FROM"); err != nil {
		return nil, err
	}
	tableTok, err := p.expect(TokenIdent, "table name")
	if err != nil {
		return nil, err
	}

	stmt := &SelectStmt{Table: tableTok.Literal, Columns: columns}

	if p.checkKeyword("WHERE") {
		p.advance()
		where, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	return stmt, nil
}

// INSERT INTO name [( col, ... )] VALUES ( value, ... )
func (p *Parser) parseInsert() (Statement, error) {
	if err := p.expectKeyword("INSERT"); err != nil {
		return nil, err
	}
	if err := p.expectKeyword("INTO"); err != nil {
		return nil, err
	}
	tableTok, err := p.expect(TokenIdent, "table name")
	if err != nil {
		return nil, err
	}

	var columns []string
	if p.check(TokenLParen) {
		p.advance()
		for {
			colTok, err := p.expect(TokenIdent, "column name")
			if err != nil {
				return nil, err
			}
			columns = append(columns, colTok.Literal)
			if p.check(TokenComma) {
				p.advance()
				continue
			}
			break
		}
		if _, err := p.expect(TokenRParen, "')'"); err != nil {
			return nil, err
		}
	}

	if err := p.expectKeyword("VALUES"); err != nil {
		return nil, err
	}
	if _, err := p.expect(TokenLParen, "'('"); err != nil {
		return nil, err
	}

	var values []Expr
	for {
		val, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		values = append(values, val)
		if p.check(TokenComma) {
			p.advance()
			continue
		}
		break
	}
	if _, err := p.expect(TokenRParen, "')'"); err != nil {
		return nil, err
	}

	return &InsertStmt{Table: tableTok.Literal, Columns: columns, Values: values}, nil
}

// UPDATE name SET col = value, ... [WHERE expr]
func (p *Parser) parseUpdate() (Statement, error) {
	if err := p.expectKeyword("UPDATE"); err != nil {
		return nil, err
	}
	tableTok, err := p.expect(TokenIdent, "table name")
	if err != nil {
		return nil, err
	}
	if err := p.expectKeyword("SET"); err != nil {
		return nil, err
	}

	var sets []SetClause
	for {
		colTok, err := p.expect(TokenIdent, "column name")
		if err != nil {
			return nil, err
		}
		opTok, err := p.expect(TokenOp, "'='")
		if err != nil {
			return nil, err
		}
		if opTok.Literal != "=" {
			return nil, fmt.Errorf("expected '=' in SET clause, got %q", opTok.Literal)
		}
		val, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		sets = append(sets, SetClause{Column: colTok.Literal, Value: val})
		if p.check(TokenComma) {
			p.advance()
			continue
		}
		break
	}

	stmt := &UpdateStmt{Table: tableTok.Literal, Set: sets}

	if p.checkKeyword("WHERE") {
		p.advance()
		where, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	return stmt, nil
}

// DELETE FROM name [WHERE expr]
func (p *Parser) parseDelete() (Statement, error) {
	if err := p.expectKeyword("DELETE"); err != nil {
		return nil, err
	}
	if err := p.expectKeyword("FROM"); err != nil {
		return nil, err
	}
	tableTok, err := p.expect(TokenIdent, "table name")
	if err != nil {
		return nil, err
	}

	stmt := &DeleteStmt{Table: tableTok.Literal}

	if p.checkKeyword("WHERE") {
		p.advance()
		where, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	return stmt, nil
}

// --- expressions (precedence climbing: OR < AND < comparison < primary) ---

func (p *Parser) parseExpr() (Expr, error) {
	return p.parseOr()
}

func (p *Parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.checkKeyword("OR") {
		p.advance()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "OR", Right: right}
	}
	return left, nil
}

func (p *Parser) parseAnd() (Expr, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.checkKeyword("AND") {
		p.advance()
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{Left: left, Op: "AND", Right: right}
	}
	return left, nil
}

func (p *Parser) parseComparison() (Expr, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}

	if p.check(TokenOp) {
		op := p.advance().Literal
		right, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Left: left, Op: op, Right: right}, nil
	}

	return left, nil
}

func (p *Parser) parsePrimary() (Expr, error) {
	tok := p.peek()

	switch tok.Type {
	case TokenLParen:
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(TokenRParen, "')'"); err != nil {
			return nil, err
		}
		return expr, nil

	case TokenInt:
		p.advance()
		v, err := strconv.Atoi(tok.Literal)
		if err != nil {
			return nil, fmt.Errorf("invalid integer literal %q", tok.Literal)
		}
		return &Literal{Value: v}, nil

	case TokenFloat:
		p.advance()
		v, err := strconv.ParseFloat(tok.Literal, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid float literal %q", tok.Literal)
		}
		return &Literal{Value: v}, nil

	case TokenString:
		p.advance()
		return &Literal{Value: tok.Literal}, nil

	case TokenKeyword:
		switch tok.Literal {
		case "TRUE":
			p.advance()
			return &Literal{Value: true}, nil
		case "FALSE":
			p.advance()
			return &Literal{Value: false}, nil
		case "NULL":
			p.advance()
			return &Literal{Value: nil}, nil
		}
		return nil, fmt.Errorf("unexpected keyword %q in expression", tok.Literal)

	case TokenIdent:
		p.advance()
		return &ColumnRef{Name: tok.Literal}, nil

	default:
		return nil, fmt.Errorf("unexpected token %q in expression", tok.Literal)
	}
}