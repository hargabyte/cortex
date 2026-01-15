package extract

import (
	"context"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

func TestComputeBodyHashFromAST_Deterministic(t *testing.T) {
	source := []byte(`package main

func Add(a, b int) int {
	return a + b
}
`)

	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	defer tree.Close()

	// Find the function declaration
	funcNode := findFunctionDeclaration(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function declaration")
	}

	hash1 := ComputeBodyHashFromAST(funcNode, source)
	hash2 := ComputeBodyHashFromAST(funcNode, source)

	if hash1 != hash2 {
		t.Errorf("Body hash not deterministic: %s != %s", hash1, hash2)
	}

	if len(hash1) != HashLength {
		t.Errorf("Hash length should be %d, got %d", HashLength, len(hash1))
	}
}

func TestComputeBodyHashFromAST_DifferentBodies(t *testing.T) {
	source1 := []byte(`package main

func Add(a, b int) int {
	return a + b
}
`)

	source2 := []byte(`package main

func Add(a, b int) int {
	return a - b
}
`)

	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree1, _ := parser.ParseCtx(context.Background(), nil, source1)
	defer tree1.Close()
	tree2, _ := parser.ParseCtx(context.Background(), nil, source2)
	defer tree2.Close()

	funcNode1 := findFunctionDeclaration(tree1.RootNode())
	funcNode2 := findFunctionDeclaration(tree2.RootNode())

	hash1 := ComputeBodyHashFromAST(funcNode1, source1)
	hash2 := ComputeBodyHashFromAST(funcNode2, source2)

	if hash1 == hash2 {
		t.Errorf("Different bodies should produce different hashes: %s == %s", hash1, hash2)
	}
}

func TestComputeBodyHashFromAST_WhitespaceInsensitive(t *testing.T) {
	// Same code with different formatting
	source1 := []byte(`package main

func Add(a, b int) int {
	return a + b
}
`)

	source2 := []byte(`package main

func Add(a, b int) int {
	return a+b
}
`)

	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree1, _ := parser.ParseCtx(context.Background(), nil, source1)
	defer tree1.Close()
	tree2, _ := parser.ParseCtx(context.Background(), nil, source2)
	defer tree2.Close()

	funcNode1 := findFunctionDeclaration(tree1.RootNode())
	funcNode2 := findFunctionDeclaration(tree2.RootNode())

	hash1 := ComputeBodyHashFromAST(funcNode1, source1)
	hash2 := ComputeBodyHashFromAST(funcNode2, source2)

	// Note: AST normalization captures structure, not whitespace
	// The AST for "a + b" and "a+b" is the same
	if hash1 != hash2 {
		t.Errorf("Whitespace differences should not affect hash: %s != %s", hash1, hash2)
	}
}

func TestComputeBodyHashFromAST_CommentInsensitive(t *testing.T) {
	source1 := []byte(`package main

func Add(a, b int) int {
	return a + b
}
`)

	source2 := []byte(`package main

func Add(a, b int) int {
	// This is a comment
	return a + b
}
`)

	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())

	tree1, _ := parser.ParseCtx(context.Background(), nil, source1)
	defer tree1.Close()
	tree2, _ := parser.ParseCtx(context.Background(), nil, source2)
	defer tree2.Close()

	funcNode1 := findFunctionDeclaration(tree1.RootNode())
	funcNode2 := findFunctionDeclaration(tree2.RootNode())

	hash1 := ComputeBodyHashFromAST(funcNode1, source1)
	hash2 := ComputeBodyHashFromAST(funcNode2, source2)

	// Comments should be ignored in hash computation
	if hash1 != hash2 {
		t.Errorf("Comments should not affect hash: %s != %s", hash1, hash2)
	}
}

func TestComputeBodyHashFromAST_NoBody(t *testing.T) {
	hash := ComputeBodyHashFromAST(nil, nil)
	if hash != "00000000" {
		t.Errorf("Nil node should return empty hash, got: %s", hash)
	}
}

func TestComputeFileHash(t *testing.T) {
	content := []byte("package main\n\nfunc main() {}")

	hash1 := ComputeFileHash(content)
	hash2 := ComputeFileHash(content)

	if hash1 != hash2 {
		t.Errorf("File hash not deterministic: %s != %s", hash1, hash2)
	}

	if len(hash1) != HashLength {
		t.Errorf("Hash length should be %d, got %d", HashLength, len(hash1))
	}

	// Different content should produce different hash
	content2 := []byte("package main\n\nfunc main() { println() }")
	hash3 := ComputeFileHash(content2)
	if hash1 == hash3 {
		t.Errorf("Different content should produce different hash")
	}
}

func TestCompareHashes(t *testing.T) {
	tests := []struct {
		name            string
		old             string
		new             string
		wantSigChanged  bool
		wantBodyChanged bool
	}{
		{
			name:            "no changes",
			old:             "abcd1234:efgh5678",
			new:             "abcd1234:efgh5678",
			wantSigChanged:  false,
			wantBodyChanged: false,
		},
		{
			name:            "signature changed",
			old:             "abcd1234:efgh5678",
			new:             "xxxx9999:efgh5678",
			wantSigChanged:  true,
			wantBodyChanged: false,
		},
		{
			name:            "body changed",
			old:             "abcd1234:efgh5678",
			new:             "abcd1234:yyyy0000",
			wantSigChanged:  false,
			wantBodyChanged: true,
		},
		{
			name:            "both changed",
			old:             "abcd1234:efgh5678",
			new:             "xxxx9999:yyyy0000",
			wantSigChanged:  true,
			wantBodyChanged: true,
		},
		{
			name:            "invalid old format",
			old:             "invalidhash",
			new:             "abcd1234:efgh5678",
			wantSigChanged:  true,
			wantBodyChanged: true,
		},
		{
			name:            "invalid new format",
			old:             "abcd1234:efgh5678",
			new:             "invalidhash",
			wantSigChanged:  true,
			wantBodyChanged: true,
		},
		{
			name:            "empty old body",
			old:             "abcd1234:",
			new:             "abcd1234:efgh5678",
			wantSigChanged:  false,
			wantBodyChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sigChanged, bodyChanged := CompareHashes(tt.old, tt.new)
			if sigChanged != tt.wantSigChanged {
				t.Errorf("sigChanged = %v, want %v", sigChanged, tt.wantSigChanged)
			}
			if bodyChanged != tt.wantBodyChanged {
				t.Errorf("bodyChanged = %v, want %v", bodyChanged, tt.wantBodyChanged)
			}
		})
	}
}

func TestFormatAndParseHashPair(t *testing.T) {
	sigHash := "abcd1234"
	bodyHash := "efgh5678"

	formatted := FormatHashPair(sigHash, bodyHash)
	if formatted != "abcd1234:efgh5678" {
		t.Errorf("FormatHashPair = %s, want abcd1234:efgh5678", formatted)
	}

	parsedSig, parsedBody := ParseHashPair(formatted)
	if parsedSig != sigHash {
		t.Errorf("ParseHashPair sig = %s, want %s", parsedSig, sigHash)
	}
	if parsedBody != bodyHash {
		t.Errorf("ParseHashPair body = %s, want %s", parsedBody, bodyHash)
	}
}

func TestParseHashPair_Invalid(t *testing.T) {
	// No colon
	sig, body := ParseHashPair("invalidhash")
	if sig != "invalidhash" || body != "" {
		t.Errorf("Invalid format should return original as sig: got %s, %s", sig, body)
	}
}

func TestParseHashPair_EmptyBody(t *testing.T) {
	sig, body := ParseHashPair("abcd1234:")
	if sig != "abcd1234" || body != "" {
		t.Errorf("Empty body should parse correctly: got sig=%s, body=%s", sig, body)
	}
}

func TestParseHashPair_EmptySig(t *testing.T) {
	sig, body := ParseHashPair(":efgh5678")
	if sig != "" || body != "efgh5678" {
		t.Errorf("Empty sig should parse correctly: got sig=%s, body=%s", sig, body)
	}
}

func TestIsEmptyHash(t *testing.T) {
	if !IsEmptyHash("00000000") {
		t.Error("00000000 should be empty hash")
	}
	if IsEmptyHash("abcd1234") {
		t.Error("abcd1234 should not be empty hash")
	}
}

func TestNormalizeSignatureForHash(t *testing.T) {
	params := []Param{
		{Name: "a", Type: "int"},
		{Name: "b", Type: "int"},
	}
	returns := []string{"int"}

	sig1 := NormalizeSignatureForHash("Add", params, returns, "")
	sig2 := NormalizeSignatureForHash("Add", params, returns, "")

	if sig1 != sig2 {
		t.Errorf("Same signature should produce same normalized form: %s != %s", sig1, sig2)
	}

	// With receiver
	sigWithReceiver := NormalizeSignatureForHash("Add", params, returns, "*Calculator")
	if sig1 == sigWithReceiver {
		t.Error("Different receivers should produce different normalized form")
	}
}

func TestNormalizeTypeForHash(t *testing.T) {
	fields := []Field{
		{Name: "ID", Type: "int"},
		{Name: "Name", Type: "string"},
	}

	type1 := NormalizeTypeForHash("User", StructKind, fields)
	type2 := NormalizeTypeForHash("User", StructKind, fields)

	if type1 != type2 {
		t.Errorf("Same type should produce same normalized form: %s != %s", type1, type2)
	}

	// Different type kind
	typeInterface := NormalizeTypeForHash("User", InterfaceKind, fields)
	if type1 == typeInterface {
		t.Error("Different type kinds should produce different normalized form")
	}
}

func TestComputeEntityHashes(t *testing.T) {
	source := []byte(`package main

func Add(a, b int) int {
	return a + b
}
`)

	parser := sitter.NewParser()
	parser.SetLanguage(golang.GetLanguage())
	tree, err := parser.ParseCtx(context.Background(), nil, source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	defer tree.Close()

	funcNode := findFunctionDeclaration(tree.RootNode())
	if funcNode == nil {
		t.Fatal("Could not find function declaration")
	}

	entity := &Entity{
		Kind:    FunctionEntity,
		Name:    "Add",
		File:    "test.go",
		Params:  []Param{{Name: "a", Type: "int"}, {Name: "b", Type: "int"}},
		Returns: []string{"int"},
	}

	ComputeEntityHashes(entity, funcNode, source)

	if entity.SigHash == "" {
		t.Error("SigHash should be computed")
	}
	if entity.BodyHash == "" {
		t.Error("BodyHash should be computed")
	}
	if len(entity.SigHash) != HashLength {
		t.Errorf("SigHash should be %d chars, got %d", HashLength, len(entity.SigHash))
	}
	if len(entity.BodyHash) != HashLength {
		t.Errorf("BodyHash should be %d chars, got %d", HashLength, len(entity.BodyHash))
	}
}

func TestEntitySignatureHash_DifferentSignatures(t *testing.T) {
	tests := []struct {
		name    string
		entity1 *Entity
		entity2 *Entity
	}{
		{
			name: "different names",
			entity1: &Entity{
				Kind:    FunctionEntity,
				Name:    "FuncA",
				Params:  []Param{{Type: "int"}},
				Returns: []string{"int"},
			},
			entity2: &Entity{
				Kind:    FunctionEntity,
				Name:    "FuncB",
				Params:  []Param{{Type: "int"}},
				Returns: []string{"int"},
			},
		},
		{
			name: "different params",
			entity1: &Entity{
				Kind:    FunctionEntity,
				Name:    "Process",
				Params:  []Param{{Type: "int"}},
				Returns: []string{"int"},
			},
			entity2: &Entity{
				Kind:    FunctionEntity,
				Name:    "Process",
				Params:  []Param{{Type: "string"}},
				Returns: []string{"int"},
			},
		},
		{
			name: "different returns",
			entity1: &Entity{
				Kind:    FunctionEntity,
				Name:    "Process",
				Params:  []Param{{Type: "int"}},
				Returns: []string{"int"},
			},
			entity2: &Entity{
				Kind:    FunctionEntity,
				Name:    "Process",
				Params:  []Param{{Type: "int"}},
				Returns: []string{"error"},
			},
		},
		{
			name: "different receiver types",
			entity1: &Entity{
				Kind:     MethodEntity,
				Name:     "Process",
				Params:   []Param{{Type: "int"}},
				Returns:  []string{"int"},
				Receiver: "*Server",
			},
			entity2: &Entity{
				Kind:     MethodEntity,
				Name:     "Process",
				Params:   []Param{{Type: "int"}},
				Returns:  []string{"int"},
				Receiver: "Server",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.entity1.ComputeHashes()
			tt.entity2.ComputeHashes()

			if tt.entity1.SigHash == tt.entity2.SigHash {
				t.Errorf("Different signatures should produce different hashes: %s == %s",
					tt.entity1.SigHash, tt.entity2.SigHash)
			}
		})
	}
}

func TestEntitySignatureHash_Deterministic(t *testing.T) {
	entity := &Entity{
		Kind:    FunctionEntity,
		Name:    "ProcessData",
		Params:  []Param{{Type: "[]byte"}},
		Returns: []string{"error"},
	}

	entity.ComputeHashes()
	hash1 := entity.SigHash

	entity.SigHash = "" // Reset
	entity.ComputeHashes()
	hash2 := entity.SigHash

	if hash1 != hash2 {
		t.Errorf("Hash not deterministic: %s != %s", hash1, hash2)
	}

	if len(hash1) != HashLength {
		t.Errorf("Hash length should be %d, got %d", HashLength, len(hash1))
	}
}

func TestEntityTypeHash(t *testing.T) {
	entity1 := &Entity{
		Kind:     TypeEntity,
		Name:     "User",
		TypeKind: StructKind,
		Fields: []Field{
			{Name: "ID", Type: "int"},
			{Name: "Name", Type: "string"},
		},
	}

	entity2 := &Entity{
		Kind:     TypeEntity,
		Name:     "User",
		TypeKind: StructKind,
		Fields: []Field{
			{Name: "ID", Type: "int"},
			{Name: "Email", Type: "string"}, // Different field
		},
	}

	entity1.ComputeHashes()
	entity2.ComputeHashes()

	if entity1.SigHash == entity2.SigHash {
		t.Errorf("Different struct fields should produce different hashes")
	}

	// Same entity should be deterministic
	hash1 := entity1.SigHash
	entity1.SigHash = ""
	entity1.ComputeHashes()
	if hash1 != entity1.SigHash {
		t.Errorf("Same entity should produce same hash: %s != %s", hash1, entity1.SigHash)
	}
}

// findFunctionDeclaration finds the first function_declaration node in the AST
func findFunctionDeclaration(root *sitter.Node) *sitter.Node {
	if root == nil {
		return nil
	}
	if root.Type() == "function_declaration" {
		return root
	}
	for i := 0; i < int(root.ChildCount()); i++ {
		if found := findFunctionDeclaration(root.Child(i)); found != nil {
			return found
		}
	}
	return nil
}
