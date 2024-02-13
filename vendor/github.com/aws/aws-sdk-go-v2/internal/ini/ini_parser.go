package ini

import (
	"fmt"
	"io"
)

// ParseState represents the current state of the parser.
type ParseState uint

// State enums for the parse table
const (
	InvalidState ParseState = iota
	// stmt -> value stmt'
	StatementState
	// stmt' -> MarkComplete | op stmt
	StatementPrimeState
	// value -> number | string | boolean | quoted_string
	ValueState
	// section -> [ section'
	OpenScopeState
	// section' -> value section_close
	SectionState
	// section_close -> ]
	CloseScopeState
	// SkipState will skip (NL WS)+
	SkipState
	// SkipTokenState will skip any token and push the previous
	// state onto the stack.
	SkipTokenState
	// comment -> # comment' | ; comment'
	// comment' -> MarkComplete | value
	CommentState
	// MarkComplete state will complete statements and move that
	// to the completed AST list
	MarkCompleteState
	// TerminalState signifies that the tokens have been fully parsed
	TerminalState
)

// parseTable is a state machine to dictate the grammar above.
var parseTable = map[ASTKind]map[TokenType]ParseState{
	ASTKindStart: {
		TokenLit:     StatementState,
		TokenSep:     OpenScopeState,
		TokenWS:      SkipTokenState,
		TokenNL:      SkipTokenState,
		TokenComment: CommentState,
		TokenNone:    TerminalState,
	},
	ASTKindCommentStatement: {
		TokenLit:     StatementState,
		TokenSep:     OpenScopeState,
		TokenWS:      SkipTokenState,
		TokenNL:      SkipTokenState,
		TokenComment: CommentState,
		TokenNone:    MarkCompleteState,
	},
	ASTKindExpr: {
		TokenOp:      StatementPrimeState,
		TokenLit:     ValueState,
		TokenSep:     OpenScopeState,
		TokenWS:      ValueState,
		TokenNL:      SkipState,
		TokenComment: CommentState,
		TokenNone:    MarkCompleteState,
	},
	ASTKindEqualExpr: {
		TokenLit: ValueState,
		TokenSep: ValueState,
		TokenOp:  ValueState,
		TokenWS:  SkipTokenState,
		TokenNL:  SkipState,
	},
	ASTKindStatement: {
		TokenLit:     SectionState,
		TokenSep:     CloseScopeState,
		TokenWS:      SkipTokenState,
		TokenNL:      SkipTokenState,
		TokenComment: CommentState,
		TokenNone:    MarkCompleteState,
	},
	ASTKindExprStatement: {
		TokenLit:     ValueState,
		TokenSep:     ValueState,
		TokenOp:      ValueState,
		TokenWS:      ValueState,
		TokenNL:      MarkCompleteState,
		TokenComment: CommentState,
		TokenNone:    TerminalState,
		TokenComma:   SkipState,
	},
	ASTKindSectionStatement: {
		TokenLit: SectionState,
		TokenOp:  SectionState,
		TokenSep: CloseScopeState,
		TokenWS:  SectionState,
		TokenNL:  SkipTokenState,
	},
	ASTKindCompletedSectionStatement: {
		TokenWS:      SkipTokenState,
		TokenNL:      SkipTokenState,
		TokenLit:     StatementState,
		TokenSep:     OpenScopeState,
		TokenComment: CommentState,
		TokenNone:    MarkCompleteState,
	},
	ASTKindSkipStatement: {
		TokenLit:     StatementState,
		TokenSep:     OpenScopeState,
		TokenWS:      SkipTokenState,
		TokenNL:      SkipTokenState,
		TokenComment: CommentState,
		TokenNone:    TerminalState,
	},
}

// ParseAST will parse input from an io.Reader using
// an LL(1) parser.
func ParseAST(r io.Reader) ([]AST, error) {
	lexer := iniLexer{}
	tokens, err := lexer.Tokenize(r)
	if err != nil {
		return []AST{}, err
	}

	return parse(tokens)
}

// ParseASTBytes will parse input from a byte slice using
// an LL(1) parser.
func ParseASTBytes(b []byte) ([]AST, error) {
	lexer := iniLexer{}
	tokens, err := lexer.tokenize(b)
	if err != nil {
		return []AST{}, err
	}

	return parse(tokens)
}

func parse(tokens []Token) ([]AST, error) {
	start := Start
	stack := newParseStack(3, len(tokens))

	stack.Push(start)
	s := newSkipper()

loop:
	for stack.Len() > 0 {
		k := stack.Pop()

		var tok Token
		if len(tokens) == 0 {
			// this occurs when all the tokens have been processed
			// but reduction of what's left on the stack needs to
			// occur.
			tok = emptyToken
		} else {
			tok = tokens[0]
		}

		step := parseTable[k.Kind][tok.Type()]
		if s.ShouldSkip(tok) {
			// being in a skip state with no tokens will break out of
			// the parse loop since there is nothing left to process.
			if len(tokens) == 0 {
				break loop
			}
			// if should skip is true, we skip the tokens until should skip is set to false.
			step = SkipTokenState
		}

		switch step {
		case TerminalState:
			// Finished parsing. Push what should be the last
			// statement to the stack. If there is anything left
			// on the stack, an error in parsing has occurred.
			if k.Kind != ASTKindStart {
				stack.MarkComplete(k)
			}
			break loop
		case SkipTokenState:
			// When skipping a token, the previous state was popped off the stack.
			// To maintain the correct state, the previous state will be pushed
			// onto the stack.
			stack.Push(k)
		case StatementState:
			if k.Kind != ASTKindStart {
				stack.MarkComplete(k)
			}
			expr := newExpression(tok)
			stack.Push(expr)
		case StatementPrimeState:
			if tok.Type() != TokenOp {
				stack.MarkComplete(k)
				continue
			}

			if k.Kind != ASTKindExpr {
				return nil, NewParseError(
					fmt.Sprintf("invalid expression: expected Expr type, but found %T type", k),
				)
			}

			k = trimSpaces(k)
			expr := newEqualExpr(k, tok)
			stack.Push(expr)
		case ValueState:
			// ValueState requires the previous state to either be an equal expression
			// or an expression statement.
			switch k.Kind {
			case ASTKindEqualExpr:
				// assigning a value to some key
				k.AppendChild(newExpression(tok))
				stack.Push(newExprStatement(k))
			case ASTKindExpr:
				k.Root.raw = append(k.Root.raw, tok.Raw()...)
				stack.Push(k)
			case ASTKindExprStatement:
				root := k.GetRoot()
				children := root.GetChildren()
				if len(children) == 0 {
					return nil, NewParseError(
						fmt.Sprintf("invalid expression: AST contains no children %s", k.Kind),
					)
				}

				rhs := children[len(children)-1]

				if rhs.Root.ValueType != QuotedStringType {
					rhs.Root.ValueType = StringType
					rhs.Root.raw = append(rhs.Root.raw, tok.Raw()...)

				}

				children[len(children)-1] = rhs
				root.SetChildren(children)

				stack.Push(k)
			}
		case OpenScopeState:
			if !runeCompare(tok.Raw(), openBrace) {
				return nil, NewParseError("expected '['")
			}
			// If OpenScopeState is not at the start, we must mark the previous ast as complete
			//
			// for example: if previous ast was a skip statement;
			// we should mark it as complete before we create a new statement
			if k.Kind != ASTKindStart {
				stack.MarkComplete(k)
			}

			stmt := newStatement()
			stack.Push(stmt)
		case CloseScopeState:
			if !runeCompare(tok.Raw(), closeBrace) {
				return nil, NewParseError("expected ']'")
			}

			k = trimSpaces(k)
			stack.Push(newCompletedSectionStatement(k))
		case SectionState:
			var stmt AST

			switch k.Kind {
			case ASTKindStatement:
				// If there are multiple literals inside of a scope declaration,
				// then the current token's raw value will be appended to the Name.
				//
				// This handles cases like [ profile default ]
				//
				// k will represent a SectionStatement with the children representing
				// the label of the section
				stmt = newSectionStatement(tok)
			case ASTKindSectionStatement:
				k.Root.raw = append(k.Root.raw, tok.Raw()...)
				stmt = k
			default:
				return nil, NewParseError(
					fmt.Sprintf("invalid statement: expected statement: %v", k.Kind),
				)
			}

			stack.Push(stmt)
		case MarkCompleteState:
			if k.Kind != ASTKindStart {
				stack.MarkComplete(k)
			}

			if stack.Len() == 0 {
				stack.Push(start)
			}
		case SkipState:
			stack.Push(newSkipStatement(k))
			s.Skip()
		case CommentState:
			if k.Kind == ASTKindStart {
				stack.Push(k)
			} else {
				stack.MarkComplete(k)
			}

			stmt := newCommentStatement(tok)
			stack.Push(stmt)
		default:
			return nil, NewParseError(
				fmt.Sprintf("invalid state with ASTKind %v and TokenType %v",
					k.Kind, tok.Type()))
		}

		if len(tokens) > 0 {
			tokens = tokens[1:]
		}
	}

	// this occurs when a statement has not been completed
	if stack.top > 1 {
		return nil, NewParseError(fmt.Sprintf("incomplete ini expression"))
	}

	// returns a sublist which exludes the start symbol
	return stack.List(), nil
}

// trimSpaces will trim spaces on the left and right hand side of
// the literal.
func trimSpaces(k AST) AST {
	// trim left hand side of spaces
	for i := 0; i < len(k.Root.raw); i++ {
		if !isWhitespace(k.Root.raw[i]) {
			break
		}

		k.Root.raw = k.Root.raw[1:]
		i--
	}

	// trim right hand side of spaces
	for i := len(k.Root.raw) - 1; i >= 0; i-- {
		if !isWhitespace(k.Root.raw[i]) {
			break
		}

		k.Root.raw = k.Root.raw[:len(k.Root.raw)-1]
	}

	return k
}
