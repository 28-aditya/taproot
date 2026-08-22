package sql

import (
	"fmt"
	"strings"
)

type TokenType int

const (
	TokenIdent TokenType = iota
	TokenKeyword
	TokenInt
	TokenFloat
	TokenString
	TokenOp // = != <> < <= > >=
	TokenStar
	TokenComma
	TokenLParen
	TokenRParen
	TokenSemicolon
	TokenEOF
)

type Token struct {
	Type    TokenType
	Literal string
}

var keywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true,
	"INSERT": true, "INTO": true, "VALUES": true,
	"UPDATE": true, "SET": true,
	"DELETE": true,
	"CREATE": true, "TABLE": true, "DROP": true,
	"DESC": true, "DESCRIBE": true,
	"SHOW": true, "TABLES": true,
	"PRIMARY": true, "KEY": true,
	"AND": true, "OR": true, "NOT": true,
	"NULL": true, "TRUE": true, "FALSE": true,
}

func Tokenize(input string) ([]Token, error) {
	var tokens []Token
	runes := []rune(input)
	i, n := 0, len(runes)

	for i < n {
		c := runes[i]

		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++

		case c == '\'' || c == '"':
			quote := c
			j := i + 1
			var sb strings.Builder
			for j < n && runes[j] != quote {
				sb.WriteRune(runes[j])
				j++
			}
			if j >= n {
				return nil, fmt.Errorf("unterminated string literal starting at position %d", i)
			}
			tokens = append(tokens, Token{Type: TokenString, Literal: sb.String()})
			i = j + 1

		case c >= '0' && c <= '9':
			j := i
			for j < n && runes[j] >= '0' && runes[j] <= '9' {
				j++
			}
			isFloat := false
			if j < n && runes[j] == '.' && j+1 < n && runes[j+1] >= '0' && runes[j+1] <= '9' {
				isFloat = true
				j++
				for j < n && runes[j] >= '0' && runes[j] <= '9' {
					j++
				}
			}
			typ := TokenInt
			if isFloat {
				typ = TokenFloat
			}
			tokens = append(tokens, Token{Type: typ, Literal: string(runes[i:j])})
			i = j

		case isIdentStart(c):
			j := i
			for j < n && isIdentPart(runes[j]) {
				j++
			}
			word := string(runes[i:j])
			upper := strings.ToUpper(word)
			if keywords[upper] {
				tokens = append(tokens, Token{Type: TokenKeyword, Literal: upper})
			} else {
				tokens = append(tokens, Token{Type: TokenIdent, Literal: word})
			}
			i = j

		case c == '*':
			tokens = append(tokens, Token{Type: TokenStar, Literal: "*"})
			i++

		case c == ',':
			tokens = append(tokens, Token{Type: TokenComma, Literal: ","})
			i++

		case c == '(':
			tokens = append(tokens, Token{Type: TokenLParen, Literal: "("})
			i++

		case c == ')':
			tokens = append(tokens, Token{Type: TokenRParen, Literal: ")"})
			i++

		case c == ';':
			tokens = append(tokens, Token{Type: TokenSemicolon, Literal: ";"})
			i++

		case c == '=' || c == '<' || c == '>' || c == '!':
			switch {
			case (c == '<' || c == '>' || c == '!') && i+1 < n && runes[i+1] == '=':
				tokens = append(tokens, Token{Type: TokenOp, Literal: string(c) + "="})
				i += 2
			case c == '<' && i+1 < n && runes[i+1] == '>':
				tokens = append(tokens, Token{Type: TokenOp, Literal: "<>"})
				i += 2
			case c == '!':
				return nil, fmt.Errorf("unexpected character '!' at position %d", i)
			default:
				tokens = append(tokens, Token{Type: TokenOp, Literal: string(c)})
				i++
			}

		default:
			return nil, fmt.Errorf("unexpected character %q at position %d", c, i)
		}
	}

	tokens = append(tokens, Token{Type: TokenEOF})
	return tokens, nil
}

func isIdentStart(c rune) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c rune) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}