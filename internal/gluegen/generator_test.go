/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package gluegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGeneratePackageGlueGen(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	tmpRoot, err := os.MkdirTemp(filepath.Join(repoRoot, "internal", "gluegen"), "tmpgen-")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpRoot)

	relPkgDir, err := filepath.Rel(repoRoot, filepath.Join(tmpRoot, "services"))
	if err != nil {
		t.Fatalf("filepath.Rel failed: %v", err)
	}

	writeFile(t, filepath.Join(tmpRoot, "services", "services.go"), strings.Join([]string{
		"package services",
		"",
		"import (",
		"\t\"reflect\"",
		"",
		"\t\"go.arpabet.com/glue\"",
		")",
		"",
		"//glue:component",
		"type explicitService struct {",
		"\tName string",
		"}",
		"",
		"type taggedService struct {",
		"\tDep *explicitService `inject:\"\"`",
		"}",
		"",
		"type product struct {",
		"\tID int",
		"}",
		"",
		"type productFactory struct {",
		"\tglue.FactoryBean",
		"}",
		"",
		"func (f *productFactory) Object() (any, error) {",
		"\treturn &product{}, nil",
		"}",
		"",
		"func (f *productFactory) ObjectType() reflect.Type {",
		"\treturn reflect.TypeOf((*product)(nil))",
		"}",
		"",
		"func (f *productFactory) ObjectName() string {",
		"\treturn \"product\"",
		"}",
		"",
		"func (f *productFactory) Singleton() bool {",
		"\treturn true",
		"}",
	}, "\n"))

	writeFile(t, filepath.Join(tmpRoot, "services", "services_test.go"), strings.Join([]string{
		"package services",
		"",
		"import (",
		"\t\"testing\"",
		"",
		"\t\"go.arpabet.com/glue\"",
		")",
		"",
		"func TestGlueGenScanner(t *testing.T) {",
		"\tctn, err := glue.New(GlueGen())",
		"\tif err != nil {",
		"\t\tt.Fatalf(\"glue.New: %v\", err)",
		"\t}",
		"\tdefer ctn.Close()",
		"",
		"\tif got := len(ctn.Lookup(\"*services.explicitService\", glue.DefaultSearchLevel)); got != 1 {",
		"\t\tt.Fatalf(\"explicitService beans = %d, want 1\", got)",
		"\t}",
		"\tif got := len(ctn.Lookup(\"*services.taggedService\", glue.DefaultSearchLevel)); got != 1 {",
		"\t\tt.Fatalf(\"taggedService beans = %d, want 1\", got)",
		"\t}",
		"\tif got := len(ctn.Lookup(\"*services.productFactory\", glue.DefaultSearchLevel)); got != 1 {",
		"\t\tt.Fatalf(\"productFactory beans = %d, want 1\", got)",
		"\t}",
		"}",
		"",
	}, "\n"))

	result, err := Generate(Options{
		Dir:      repoRoot,
		Patterns: []string{"./" + filepath.ToSlash(relPkgDir)},
		Write:    true,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	genPath := filepath.Join(tmpRoot, "services", generatedFileName)
	content, err := os.ReadFile(genPath)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	text := string(content)
	for _, want := range []string{
		"func GlueGen() glue.Scanner",
		"new(explicitService)",
		"new(taggedService)",
		"new(productFactory)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated file missing %q:\n%s", want, text)
		}
	}
	if len(result.Generated) != 1 || result.Generated[0] != genPath {
		t.Fatalf("generated files = %v, want [%s]", result.Generated, genPath)
	}

	cmd := exec.Command("go", "test", "./"+filepath.ToSlash(relPkgDir))
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go test generated package failed: %v\n%s", err, string(out))
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
