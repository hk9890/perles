package bql

import (
	"testing"

	"github.com/zjrosen/perles/internal/ui/shared/vimtextarea"
)

// verifyInterfaceCompliance ensures BQLSyntaxLexer implements SyntaxLexer
var _ vimtextarea.SyntaxLexer = (*BQLSyntaxLexer)(nil)

func TestBQLSyntaxLexer_Tokenize(t *testing.T) {
	lexer := NewSyntaxLexer()

	tests := []struct {
		name     string
		input    string
		wantLen  int
		validate func(t *testing.T, tokens []vimtextarea.SyntaxToken)
	}{
		{
			name:    "empty string returns nil",
			input:   "",
			wantLen: 0,
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				if tokens != nil {
					t.Errorf("expected nil, got %v", tokens)
				}
			},
		},
		{
			name:    "simple query: status = open",
			input:   "status = open",
			wantLen: 2, // "status" (field) and "=" (operator); "open" is plain text (value after operator)
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				// First token: "status" at position 0-6
				if tokens[0].Start != 0 || tokens[0].End != 6 {
					t.Errorf("token 0: expected Start=0, End=6, got Start=%d, End=%d", tokens[0].Start, tokens[0].End)
				}
				// Second token: "=" at position 7-8
				if tokens[1].Start != 7 || tokens[1].End != 8 {
					t.Errorf("token 1: expected Start=7, End=8, got Start=%d, End=%d", tokens[1].Start, tokens[1].End)
				}
			},
		},
		{
			name:    "keyword highlighting: status = open and priority < 2",
			input:   "status = open and priority < 2",
			wantLen: 6, // "status", "=", "and", "priority", "<", "2" (number is styled)
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				// Check "and" keyword is present (should be around position 14-17)
				found := false
				for _, tok := range tokens {
					if tok.Start == 14 && tok.End == 17 {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("keyword 'and' not found at expected position 14-17")
				}
			},
		},
		{
			name:    "string literal highlighting",
			input:   `title ~ "bug"`,
			wantLen: 3, // "title", "~", and "\"bug\""
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				// String token should include quotes: position 8-13
				stringTok := tokens[2]
				if stringTok.Start != 8 || stringTok.End != 13 {
					t.Errorf("string token: expected Start=8, End=13, got Start=%d, End=%d", stringTok.Start, stringTok.End)
				}
			},
		},
		{
			name:    "token positions accuracy",
			input:   "a = 1",
			wantLen: 3, // "a", "=", "1" (number is always styled)
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				// "a" at 0-1
				if tokens[0].Start != 0 || tokens[0].End != 1 {
					t.Errorf("token 0 'a': expected Start=0, End=1, got Start=%d, End=%d", tokens[0].Start, tokens[0].End)
				}
				// "=" at 2-3
				if tokens[1].Start != 2 || tokens[1].End != 3 {
					t.Errorf("token 1 '=': expected Start=2, End=3, got Start=%d, End=%d", tokens[1].Start, tokens[1].End)
				}
				// "1" at 4-5
				if tokens[2].Start != 4 || tokens[2].End != 5 {
					t.Errorf("token 2 '1': expected Start=4, End=5, got Start=%d, End=%d", tokens[2].Start, tokens[2].End)
				}
			},
		},
		{
			name:    "context-aware field vs value: assignee = john",
			input:   "assignee = john",
			wantLen: 2, // "assignee" (field), "=" (operator); "john" is plain text
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				// "assignee" at 0-8
				if tokens[0].Start != 0 || tokens[0].End != 8 {
					t.Errorf("field token: expected Start=0, End=8, got Start=%d, End=%d", tokens[0].Start, tokens[0].End)
				}
				// Verify we don't have a token for "john" (position 11-15)
				for _, tok := range tokens {
					if tok.Start == 11 {
						t.Errorf("unexpected token at position 11 - 'john' should be plain text")
					}
				}
			},
		},
		{
			name:    "IN clause highlighting",
			input:   "status in (open, closed)",
			wantLen: 5, // "status", "in", "(", ",", ")"
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				// "in" keyword at position 7-9
				found := false
				for _, tok := range tokens {
					if tok.Start == 7 && tok.End == 9 {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("keyword 'in' not found at expected position 7-9")
				}
				// Values "open" and "closed" should NOT be tokenized (plain text)
				for _, tok := range tokens {
					if tok.Start == 11 || tok.Start == 17 {
						t.Errorf("unexpected token at position %d - values in IN clause should be plain", tok.Start)
					}
				}
			},
		},
		{
			name:    "partial query: status =",
			input:   "status =",
			wantLen: 2, // "status" and "="
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				if tokens[0].Start != 0 || tokens[0].End != 6 {
					t.Errorf("token 0: expected Start=0, End=6, got Start=%d, End=%d", tokens[0].Start, tokens[0].End)
				}
				if tokens[1].Start != 7 || tokens[1].End != 8 {
					t.Errorf("token 1: expected Start=7, End=8, got Start=%d, End=%d", tokens[1].Start, tokens[1].End)
				}
			},
		},
		{
			name:    "just whitespace",
			input:   "   ",
			wantLen: 0,
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				if len(tokens) != 0 {
					t.Errorf("expected empty slice for whitespace, got %d tokens", len(tokens))
				}
			},
		},
		{
			name:    "order by clause",
			input:   "order by priority desc",
			wantLen: 4, // "order", "by", "priority" (field), "desc"
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				// "order" at 0-5
				if tokens[0].Start != 0 || tokens[0].End != 5 {
					t.Errorf("'order': expected Start=0, End=5, got Start=%d, End=%d", tokens[0].Start, tokens[0].End)
				}
				// "by" at 6-8
				if tokens[1].Start != 6 || tokens[1].End != 8 {
					t.Errorf("'by': expected Start=6, End=8, got Start=%d, End=%d", tokens[1].Start, tokens[1].End)
				}
				// "desc" at 18-22
				if tokens[3].Start != 18 || tokens[3].End != 22 {
					t.Errorf("'desc': expected Start=18, End=22, got Start=%d, End=%d", tokens[3].Start, tokens[3].End)
				}
			},
		},
		{
			name:    "boolean literal",
			input:   "active = true",
			wantLen: 3, // "active", "=", "true" (literal is always styled)
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				// "true" at position 9-13 should be styled
				trueTok := tokens[2]
				if trueTok.Start != 9 || trueTok.End != 13 {
					t.Errorf("'true': expected Start=9, End=13, got Start=%d, End=%d", trueTok.Start, trueTok.End)
				}
			},
		},
		{
			name:    "numeric literal",
			input:   "priority < 5",
			wantLen: 3, // "priority", "<", "5" (numbers are always styled)
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				// "5" at position 11-12
				numTok := tokens[2]
				if numTok.Start != 11 || numTok.End != 12 {
					t.Errorf("'5': expected Start=11, End=12, got Start=%d, End=%d", numTok.Start, numTok.End)
				}
			},
		},
		{
			name:    "expand clause with star",
			input:   "expand depth *",
			wantLen: 3, // "expand", "depth", "*"
			validate: func(t *testing.T, tokens []vimtextarea.SyntaxToken) {
				// "*" at position 13-14
				if tokens[2].Start != 13 || tokens[2].End != 14 {
					t.Errorf("'*': expected Start=13, End=14, got Start=%d, End=%d", tokens[2].Start, tokens[2].End)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := lexer.Tokenize(tt.input)

			// Check token count
			if tt.wantLen > 0 && len(tokens) != tt.wantLen {
				t.Errorf("token count: expected %d, got %d", tt.wantLen, len(tokens))
				for i, tok := range tokens {
					t.Logf("  token[%d]: Start=%d, End=%d, text=%q", i, tok.Start, tok.End, tt.input[tok.Start:tok.End])
				}
			}

			// Run custom validation
			if tt.validate != nil {
				tt.validate(t, tokens)
			}
		})
	}
}

func TestBQLSyntaxLexer_TokensAreSorted(t *testing.T) {
	lexer := NewSyntaxLexer()
	input := "status = open and priority < 2 order by created_at desc"

	tokens := lexer.Tokenize(input)

	// Verify tokens are sorted by Start position (ascending)
	for i := 1; i < len(tokens); i++ {
		if tokens[i].Start < tokens[i-1].Start {
			t.Errorf("tokens not sorted: token[%d].Start=%d < token[%d].Start=%d",
				i, tokens[i].Start, i-1, tokens[i-1].Start)
		}
	}
}

func TestBQLSyntaxLexer_TokensNonOverlapping(t *testing.T) {
	lexer := NewSyntaxLexer()
	input := "status in (open, closed) and priority >= 1"

	tokens := lexer.Tokenize(input)

	// Verify tokens don't overlap
	for i := 1; i < len(tokens); i++ {
		if tokens[i].Start < tokens[i-1].End {
			t.Errorf("tokens overlap: token[%d] ends at %d, token[%d] starts at %d",
				i-1, tokens[i-1].End, i, tokens[i].Start)
		}
	}
}

func TestBQLSyntaxLexer_StringWithSingleQuotes(t *testing.T) {
	lexer := NewSyntaxLexer()
	input := `title ~ 'test'`

	tokens := lexer.Tokenize(input)

	// Should have 3 tokens: "title", "~", "'test'"
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}

	// String token with single quotes at position 8-14
	stringTok := tokens[2]
	if stringTok.Start != 8 || stringTok.End != 14 {
		t.Errorf("string token: expected Start=8, End=14, got Start=%d, End=%d",
			stringTok.Start, stringTok.End)
	}
}

func TestBQLSyntaxLexer_UnterminatedString(t *testing.T) {
	lexer := NewSyntaxLexer()
	input := `title ~ "unterminated`

	tokens := lexer.Tokenize(input)

	// Should still produce tokens without panicking
	if len(tokens) < 2 {
		t.Errorf("expected at least 2 tokens for partial string, got %d", len(tokens))
	}

	// Verify no token extends past the input length
	for i, tok := range tokens {
		if tok.End > len(input) {
			t.Errorf("token[%d] End=%d exceeds input length %d", i, tok.End, len(input))
		}
	}
}

func TestBQLSyntaxLexer_NotOperator(t *testing.T) {
	lexer := NewSyntaxLexer()
	input := "not status = closed"

	tokens := lexer.Tokenize(input)

	// Should have: "not", "status", "="; "closed" is plain
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}

	// "not" at position 0-3
	if tokens[0].Start != 0 || tokens[0].End != 3 {
		t.Errorf("'not': expected Start=0, End=3, got Start=%d, End=%d",
			tokens[0].Start, tokens[0].End)
	}

	// After "not", "status" should be styled as a field (afterOperator was reset)
	if tokens[1].Start != 4 || tokens[1].End != 10 {
		t.Errorf("'status': expected Start=4, End=10, got Start=%d, End=%d",
			tokens[1].Start, tokens[1].End)
	}
}

func TestBQLSyntaxLexer_ContainsOperator(t *testing.T) {
	lexer := NewSyntaxLexer()
	input := "title ~ bug"

	tokens := lexer.Tokenize(input)

	// Should have: "title", "~"; "bug" is plain (value after operator)
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}

	// "~" at position 6-7
	if tokens[1].Start != 6 || tokens[1].End != 7 {
		t.Errorf("'~': expected Start=6, End=7, got Start=%d, End=%d",
			tokens[1].Start, tokens[1].End)
	}
}

func TestBQLSyntaxLexer_NotContainsOperator(t *testing.T) {
	lexer := NewSyntaxLexer()
	input := "title !~ spam"

	tokens := lexer.Tokenize(input)

	// Should have: "title", "!~"; "spam" is plain
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}

	// "!~" at position 6-8
	if tokens[1].Start != 6 || tokens[1].End != 8 {
		t.Errorf("'!~': expected Start=6, End=8, got Start=%d, End=%d",
			tokens[1].Start, tokens[1].End)
	}
}
