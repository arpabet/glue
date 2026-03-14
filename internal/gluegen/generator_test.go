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

func TestGenerateDecoratorProxy(t *testing.T) {
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

	relPkgDir, err := filepath.Rel(repoRoot, filepath.Join(tmpRoot, "app"))
	if err != nil {
		t.Fatalf("filepath.Rel failed: %v", err)
	}

	writeFile(t, filepath.Join(tmpRoot, "app", "app.go"), strings.Join([]string{
		"package app",
		"",
		"//glue:decorator",
		"type Greeter interface {",
		"\tGreet(name string) string",
		"\tGreetAll(names []string) ([]string, error)",
		"}",
		"",
		"type greeterImpl struct{}",
		"",
		"func (g *greeterImpl) Greet(name string) string {",
		"\treturn \"hello \" + name",
		"}",
		"",
		"func (g *greeterImpl) GreetAll(names []string) ([]string, error) {",
		"\tout := make([]string, len(names))",
		"\tfor i, n := range names {",
		"\t\tout[i] = g.Greet(n)",
		"\t}",
		"\treturn out, nil",
		"}",
	}, "\n"))

	writeFile(t, filepath.Join(tmpRoot, "app", "app_test.go"), strings.Join([]string{
		"package app",
		"",
		"import (",
		"\t\"reflect\"",
		"\t\"testing\"",
		")",
		"",
		"func TestProxy(t *testing.T) {",
		"\tvar impl Greeter = &greeterImpl{}",
		"\tproxy := NewGreeterProxy(impl)",
		"",
		"\t// proxy delegates to impl",
		"\tif got := proxy.Greet(\"world\"); got != \"hello world\" {",
		"\t\tt.Fatalf(\"Greet = %q, want %q\", got, \"hello world\")",
		"\t}",
		"",
		"\t// wrap with interceptor",
		"\tvar calls []string",
		"\tWrapGreeterProxy(proxy, func(method string, args []reflect.Value, next func([]reflect.Value) []reflect.Value) []reflect.Value {",
		"\t\tcalls = append(calls, method)",
		"\t\treturn next(args)",
		"\t})",
		"",
		"\tproxy.Greet(\"alice\")",
		"\tproxy.GreetAll([]string{\"bob\"})",
		"",
		"\tif len(calls) != 2 || calls[0] != \"Greet\" || calls[1] != \"GreetAll\" {",
		"\t\tt.Fatalf(\"calls = %v, want [Greet GreetAll]\", calls)",
		"\t}",
		"",
		"\t// verify proxy implements the interface",
		"\tvar _ Greeter = proxy",
		"}",
	}, "\n"))

	result, err := Generate(Options{
		Dir:      repoRoot,
		Patterns: []string{"./" + filepath.ToSlash(relPkgDir)},
		Write:    true,
	})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	genPath := filepath.Join(tmpRoot, "app", generatedFileName)
	content, err := os.ReadFile(genPath)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	text := string(content)

	for _, want := range []string{
		"type GreeterProxy struct",
		"DoGreet",
		"DoGreetAll",
		"func (p *GreeterProxy) Greet(",
		"func (p *GreeterProxy) GreetAll(",
		"func NewGreeterProxy(",
		"func WrapGreeterProxy(",
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
