package vtc

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/perbu/GTest/pkg/logging"
	"github.com/perbu/GTest/pkg/util"
)

// Token types
const (
	TokenEOF        = "EOF"
	TokenCommand    = "COMMAND"
	TokenString     = "STRING"
	TokenLBrace     = "LBRACE"
	TokenRBrace     = "RBRACE"
	TokenComment    = "COMMENT"
	TokenNewline    = "NEWLINE"
	TokenIdentifier = "IDENTIFIER"
)

// Token represents a lexical token
type Token struct {
	Type  string
	Value string
	Line  int
	Col   int
}

// Node represents an AST node
type Node struct {
	Type     string   // "vtest", "command", "block", etc.
	Name     string   // Command name or identifier
	Args     []string // Command arguments
	Children []*Node  // Child nodes
	Line     int      // Source line number
}

// Parser parses VTC files
type Parser struct {
	reader  *bufio.Reader
	line    int
	macros  *MacroStore
	logger  *logging.Logger
	current rune
	tokens  []Token
	pos     int
}

// NewParser creates a new VTC parser
func NewParser(r io.Reader, macros *MacroStore, logger *logging.Logger) *Parser {
	if macros == nil {
		macros = NewMacroStore()
	}
	return &Parser{
		reader: bufio.NewReader(r),
		line:   1,
		macros: macros,
		logger: logger,
		tokens: []Token{},
		pos:    0,
	}
}

// Parse parses the VTC file and returns the AST root
func (p *Parser) Parse() (*Node, error) {
	// Read and tokenize the entire file
	if err := p.tokenize(); err != nil {
		return nil, err
	}

	// Build the AST
	root := &Node{
		Type:     "root",
		Children: []*Node{},
	}

	for !p.isEOF() {
		node, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		if node != nil {
			root.Children = append(root.Children, node)
		}
	}

	return root, nil
}

// tokenize reads the file and creates tokens
func (p *Parser) tokenize() error {
	scanner := bufio.NewScanner(p.reader)
	lineNum := 0
	var continuedLine string

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Handle line continuation
		if strings.HasSuffix(strings.TrimRight(line, " \t"), "\\") {
			// Remove trailing \ and accumulate
			line = strings.TrimSuffix(strings.TrimRight(line, " \t"), "\\")
			continuedLine += line + " "
			continue
		}

		// Add accumulated line
		if continuedLine != "" {
			line = continuedLine + line
			continuedLine = ""
		}

		// Strip comments
		line = util.StripComments(line)
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Tokenize this line
		if err := p.tokenizeLine(line, lineNum); err != nil {
			return fmt.Errorf("line %d: %v", lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Add EOF token
	p.tokens = append(p.tokens, Token{Type: TokenEOF, Line: lineNum})

	return nil
}

// tokenizeLine tokenizes a single line
func (p *Parser) tokenizeLine(line string, lineNum int) error {
	// For Phase 1, we skip macro expansion if macros are undefined
	// They will be expanded during execution in later phases
	// Just keep the ${name} as-is for now
	// Macro expansion is optional in the parser

	i := 0
	col := 0
	isFirstToken := true

	for i < len(line) {
		c := line[i]

		// Skip whitespace
		if c == ' ' || c == '\t' {
			i++
			col++
			continue
		}

		// Handle ${...} macro references - treat as a single identifier
		if c == '$' && i+1 < len(line) && line[i+1] == '{' {
			// Find the closing }
			j := i + 2
			for j < len(line) && line[j] != '}' {
				j++
			}
			if j < len(line) {
				// Include the entire ${...} as one token
				value := line[i : j+1]
				p.tokens = append(p.tokens, Token{Type: TokenIdentifier, Value: value, Line: lineNum, Col: col})
				i = j + 1
				col += j - i + 1
				isFirstToken = false
				continue
			}
		}

		// Handle braces (but not as part of ${...})
		if c == '{' {
			p.tokens = append(p.tokens, Token{Type: TokenLBrace, Value: "{", Line: lineNum, Col: col})
			i++
			col++
			isFirstToken = false
			continue
		}

		if c == '}' {
			p.tokens = append(p.tokens, Token{Type: TokenRBrace, Value: "}", Line: lineNum, Col: col})
			i++
			col++
			isFirstToken = false // After }, tokens are arguments, not commands
			continue
		}

		// Handle quoted strings
		if c == '"' {
			// Find closing quote
			j := i + 1
			for j < len(line) && line[j] != '"' {
				if line[j] == '\\' {
					j++ // Skip escaped character
				}
				j++
			}
			if j >= len(line) {
				return fmt.Errorf("unterminated string at column %d", col)
			}
			value := line[i+1 : j]
			p.tokens = append(p.tokens, Token{Type: TokenString, Value: value, Line: lineNum, Col: col})
			i = j + 1
			col += j - i + 1
			isFirstToken = false
			continue
		}

		// Handle identifiers/commands
		j := i
		for j < len(line) && !isDelimiter(line[j]) {
			j++
		}
		if j > i {
			value := line[i:j]
			// First token on a line is a command, rest are identifiers
			tokenType := TokenIdentifier
			if isFirstToken {
				tokenType = TokenCommand
				isFirstToken = false
			}
			p.tokens = append(p.tokens, Token{Type: tokenType, Value: value, Line: lineNum, Col: col})
			col += j - i
			i = j
			continue
		}

		// Unknown character, skip
		i++
		col++
	}

	return nil
}

// isDelimiter checks if a character is a delimiter
func isDelimiter(c byte) bool {
	return c == ' ' || c == '\t' || c == '{' || c == '}' || c == '"'
}

// parseStatement parses a single statement
func (p *Parser) parseStatement() (*Node, error) {
	tok := p.peek()
	if tok.Type == TokenEOF {
		return nil, nil
	}

	// Check for vtest declaration
	if tok.Type == TokenCommand && tok.Value == "vtest" {
		return p.parseVTest()
	}

	// Parse as a command
	return p.parseCommand()
}

// parseVTest parses a vtest declaration
func (p *Parser) parseVTest() (*Node, error) {
	p.consume() // consume "vtest"

	nameToken := p.peek()
	if nameToken.Type != TokenString && nameToken.Type != TokenIdentifier {
		return nil, fmt.Errorf("line %d: expected test name after 'vtest'", nameToken.Line)
	}

	name := nameToken.Value
	p.consume()

	return &Node{
		Type: "vtest",
		Name: name,
		Line: nameToken.Line,
	}, nil
}

// parseCommand parses a command with arguments and optional block
func (p *Parser) parseCommand() (*Node, error) {
	cmdToken := p.peek()
	if cmdToken.Type != TokenCommand && cmdToken.Type != TokenIdentifier {
		return nil, nil
	}

	p.consume()

	node := &Node{
		Type: "command",
		Name: cmdToken.Value,
		Args: []string{},
		Line: cmdToken.Line,
	}

	// Collect arguments until we hit a brace or EOF
	for {
		tok := p.peek()
		if tok.Type == TokenEOF || tok.Type == TokenLBrace || tok.Type == TokenRBrace {
			break
		}

		if tok.Type == TokenCommand {
			// Next command, stop here
			break
		}

		if tok.Type == TokenString || tok.Type == TokenIdentifier {
			node.Args = append(node.Args, tok.Value)
			p.consume()
		} else {
			p.consume() // Skip unknown tokens
		}
	}

	// Check for a block
	if p.peek().Type == TokenLBrace {
		p.consume() // consume {

		// Parse block contents
		for p.peek().Type != TokenRBrace && p.peek().Type != TokenEOF {
			child, err := p.parseCommand()
			if err != nil {
				return nil, err
			}
			if child != nil {
				node.Children = append(node.Children, child)
			}
		}

		if p.peek().Type != TokenRBrace {
			return nil, fmt.Errorf("line %d: expected '}' to close block", cmdToken.Line)
		}
		p.consume() // consume }

		// After closing block, continue parsing arguments (e.g., "server s1 {...} -start")
		for {
			tok := p.peek()
			if tok.Type == TokenEOF || tok.Type == TokenLBrace || tok.Type == TokenRBrace {
				break
			}

			if tok.Type == TokenCommand {
				// Next command, stop here
				break
			}

			if tok.Type == TokenString || tok.Type == TokenIdentifier {
				node.Args = append(node.Args, tok.Value)
				p.consume()
			} else {
				p.consume() // Skip unknown tokens
			}
		}
	}

	return node, nil
}

// peek returns the current token without consuming it
func (p *Parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

// consume advances to the next token
func (p *Parser) consume() {
	if p.pos < len(p.tokens) {
		p.pos++
	}
}

// isEOF checks if we're at the end of tokens
func (p *Parser) isEOF() bool {
	return p.pos >= len(p.tokens) || p.peek().Type == TokenEOF
}

// DumpAST prints the AST for debugging
func DumpAST(node *Node, indent int) {
	if node == nil {
		return
	}

	prefix := strings.Repeat("  ", indent)
	fmt.Printf("%s%s", prefix, node.Type)
	if node.Name != "" {
		fmt.Printf(" '%s'", node.Name)
	}
	if len(node.Args) > 0 {
		fmt.Printf(" args=%v", node.Args)
	}
	fmt.Printf("\n")

	for _, child := range node.Children {
		DumpAST(child, indent+1)
	}
}
