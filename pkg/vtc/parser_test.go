package vtc

import (
	"strings"
	"testing"
)

func TestParser_Simple(t *testing.T) {
	input := `vtest "test name"`
	p := NewParser(strings.NewReader(input), nil, nil)

	root, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if root == nil {
		t.Fatal("Expected root node")
	}

	if len(root.Children) != 1 {
		t.Fatalf("Expected 1 child, got %d", len(root.Children))
	}

	vtestNode := root.Children[0]
	if vtestNode.Type != "vtest" {
		t.Errorf("Expected type 'vtest', got '%s'", vtestNode.Type)
	}

	if vtestNode.Name != "test name" {
		t.Errorf("Expected name 'test name', got '%s'", vtestNode.Name)
	}
}

func TestParser_CommandWithArgs(t *testing.T) {
	input := `server s1 -start`
	p := NewParser(strings.NewReader(input), nil, nil)

	root, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(root.Children) != 1 {
		t.Fatalf("Expected 1 child, got %d", len(root.Children))
	}

	cmd := root.Children[0]
	if cmd.Type != "command" {
		t.Errorf("Expected type 'command', got '%s'", cmd.Type)
	}

	if cmd.Name != "server" {
		t.Errorf("Expected name 'server', got '%s'", cmd.Name)
	}

	if len(cmd.Args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(cmd.Args))
	}

	if cmd.Args[0] != "s1" {
		t.Errorf("Expected arg 's1', got '%s'", cmd.Args[0])
	}

	if cmd.Args[1] != "-start" {
		t.Errorf("Expected arg '-start', got '%s'", cmd.Args[1])
	}
}

func TestParser_Block(t *testing.T) {
	input := `server s1 {
	rxreq
	txresp
}`
	p := NewParser(strings.NewReader(input), nil, nil)

	root, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(root.Children) != 1 {
		t.Fatalf("Expected 1 child, got %d", len(root.Children))
	}

	server := root.Children[0]
	if server.Name != "server" {
		t.Errorf("Expected name 'server', got '%s'", server.Name)
	}

	if len(server.Children) != 2 {
		t.Fatalf("Expected 2 children in block, got %d", len(server.Children))
	}

	if server.Children[0].Name != "rxreq" {
		t.Errorf("Expected first child 'rxreq', got '%s'", server.Children[0].Name)
	}

	if server.Children[1].Name != "txresp" {
		t.Errorf("Expected second child 'txresp', got '%s'", server.Children[1].Name)
	}
}

func TestParser_Comments(t *testing.T) {
	input := `# This is a comment
vtest "test"
# Another comment
server s1 -start  # inline comment`
	p := NewParser(strings.NewReader(input), nil, nil)

	root, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Should only have vtest and server nodes, no comments
	if len(root.Children) != 2 {
		t.Fatalf("Expected 2 children, got %d", len(root.Children))
	}
}

func TestParser_LineContinuation(t *testing.T) {
	input := `txresp -hdr foo bar \
	-hdr baz qux`
	p := NewParser(strings.NewReader(input), nil, nil)

	root, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(root.Children) != 1 {
		t.Fatalf("Expected 1 child, got %d", len(root.Children))
	}

	cmd := root.Children[0]
	if cmd.Name != "txresp" {
		t.Errorf("Expected name 'txresp', got '%s'", cmd.Name)
	}

	// Should have all arguments combined
	expectedArgs := []string{"-hdr", "foo", "bar", "-hdr", "baz", "qux"}
	if len(cmd.Args) != len(expectedArgs) {
		t.Fatalf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
	}

	for i, exp := range expectedArgs {
		if cmd.Args[i] != exp {
			t.Errorf("Arg %d: expected '%s', got '%s'", i, exp, cmd.Args[i])
		}
	}
}

func TestParser_MacroExpansion(t *testing.T) {
	macros := NewMacroStore()
	macros.Define("s1_sock", "localhost:8080")

	input := `client c1 -connect ${s1_sock}`
	p := NewParser(strings.NewReader(input), macros, nil)

	root, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(root.Children) != 1 {
		t.Fatalf("Expected 1 child, got %d", len(root.Children))
	}

	cmd := root.Children[0]
	if len(cmd.Args) != 3 {
		t.Fatalf("Expected 3 args, got %d: %v", len(cmd.Args), cmd.Args)
	}

	if cmd.Args[0] != "c1" {
		t.Errorf("Expected arg 0 to be 'c1', got '%s'", cmd.Args[0])
	}

	if cmd.Args[1] != "-connect" {
		t.Errorf("Expected arg 1 to be '-connect', got '%s'", cmd.Args[1])
	}

	// In Phase 1, macros are kept as-is in the AST
	// They will be expanded during execution in later phases
	if cmd.Args[2] != "${s1_sock}" {
		t.Errorf("Expected arg 2 to be '${s1_sock}', got '%s'", cmd.Args[2])
	}
}
