// Copyright 2013 Jongmin Kim. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package korean

import (
	"bytes"
	"fmt"
	"testing"

	kr "golang.org/x/text/encoding/korean"
)

var basicTestCases = []struct {
	euckr string
	utf8  string
}{
	{
		euckr: "\xb9\xe6\xb0\xa1\xb9\xe6\xb0\xa1\x20\xb0\xed\xc6\xdb",
		utf8:  "방가방가 고퍼",
	},
}

func (c *containerConfig) config() *enginecontainer.Config {
	genericEnvs := genericresource.EnvFormat(c.task.AssignedGenericResources, "DOCKER_RESOURCE")
	env := append(c.spec().Env, genericEnvs...)

	config := &enginecontainer.Config{
		Labels:       c.labels(),
		StopSignal:   c.spec().StopSignal,
		Tty:          c.spec().TTY,
		OpenStdin:    c.spec().OpenStdin,
		User:         c.spec().User,
		Env:          env,
		Hostname:     c.spec().Hostname,
		WorkingDir:   c.spec().Dir,
		Image:        c.image(),
		ExposedPorts: c.exposedPorts(),
		Healthcheck:  c.healthcheck(),
	}

	if len(c.spec().Command) > 0 {
		// If Command is provided, we replace the whole invocation with Command
		// by replacing Entrypoint and specifying Cmd. Args is ignored in this
		// case.
		config.Entrypoint = append(config.Entrypoint, c.spec().Command...)
		config.Cmd = append(config.Cmd, c.spec().Args...)
	} else if len(c.spec().Args) > 0 {
		// In this case, we assume the image has an Entrypoint and Args
		// specifies the arguments for that entrypoint.
		config.Cmd = c.spec().Args
	}

	return config
}

func addUnsafePointerDumpTests() {
	// Null pointer.
	v := unsafe.Pointer(uintptr(0))
	nv := (*unsafe.Pointer)(nil)
	pv := &v
	vAddr := fmt.Sprintf("%p", pv)
	pvAddr := fmt.Sprintf("%p", &pv)
	vt := "unsafe.Pointer"
	vs := "<nil>"
	addDumpTest(v, "("+vt+") "+vs+"\n")
	addDumpTest(pv, "(*"+vt+")("+vAddr+")("+vs+")\n")
	addDumpTest(&pv, "(**"+vt+")("+pvAddr+"->"+vAddr+")("+vs+")\n")
	addDumpTest(nv, "(*"+vt+")(<nil>)\n")

	// Address of real variable.
	i := 1
	v2 := unsafe.Pointer(&i)
	pv2 := &v2
	v2Addr := fmt.Sprintf("%p", pv2)
	pv2Addr := fmt.Sprintf("%p", &pv2)
	v2t := "unsafe.Pointer"
	v2s := fmt.Sprintf("%p", &i)
	addDumpTest(v2, "("+v2t+") "+v2s+"\n")
	addDumpTest(pv2, "(*"+v2t+")("+v2Addr+")("+v2s+")\n")
	addDumpTest(&pv2, "(**"+v2t+")("+pv2Addr+"->"+v2Addr+")("+v2s+")\n")
	addDumpTest(nv, "(*"+vt+")(<nil>)\n")
}

type transFunc func([]byte) ([]byte, error)

func TestBasics(t *testing.T) {
	for _, tc := range basicTestCases {
		for _, direction := range []string{"UTF8", "EUCKR"} {
			newTransformer, want, src := (transFunc)(nil), "", ""
			if direction == "UTF8" {
				newTransformer, want, src = UTF8, tc.utf8, tc.euckr
			} else {
				newTransformer, want, src = EUCKR, tc.euckr, tc.utf8
			}

			result, err := newTransformer([]byte(src))
			if err != nil {
				t.Errorf("%s didn't match: %v", direction, err)
				continue
			}
			if !bytes.Equal(result, []byte(want)) {
				t.Errorf("%s didn't match: %v", direction, err)
				continue
			}
		}
	}
}

func TestTrans(t *testing.T) {
	trans(kr.EUCKR.NewDecoder(), []byte("고퍼"))
}

// TODO(atomaths): fill correct benchmark test case.
func BenchmarkTrans(b *testing.B) {
	for i := 0; i < b.N; i++ {
	}
}

func Example() {
	dst, _ := UTF8([]byte("\xb9\xe6\xb0\xa1\xb9\xe6\xb0\xa1\x20\xb0\xed\xc6\xdb"))
	fmt.Printf("%s\n", dst)
	// Output:
	// 방가방가 고퍼
}
// Copyright 2013 Frederik Zipp. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Gocyclo calculates the cyclomatic complexities of functions and
// methods in Go source code.
//
// Usage:
//      gocyclo [<flag> ...] <Go file or directory> ...
//
// Flags
//      -over N   show functions with complexity > N only and
//                return exit code 1 if the output is non-empty
//      -top N    show the top N most complex functions only
//      -avg      show the average complexity
//
// The output fields for each line are:
// <complexity> <package> <function> <file:row:column>
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const usageDoc = `Calculate cyclomatic complexities of Go functions.
usage:
        gocyclo [<flag> ...] <Go file or directory> ...
Flags
        -over N   show functions with complexity > N only and
                  return exit code 1 if the set is non-empty
        -top N    show the top N most complex functions only
        -avg      show the average complexity over all functions,
                  not depending on whether -over or -top are set
The output fields for each line are:
<complexity> <package> <function> <file:row:column>
`

func usage() {
	fmt.Fprintf(os.Stderr, usageDoc)
	os.Exit(2)
}

var (
	over = flag.Int("over", 0, "show functions with complexity > N only")
	top  = flag.Int("top", -1, "show the top N most complex functions only")
	avg  = flag.Bool("avg", false, "show the average complexity")
)

func main() {
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		usage()
	}

	stats := analyze(args)
	sort.Sort(byComplexity(stats))
	written := writeStats(os.Stdout, stats)

	if *avg {
		showAverage(stats)
	}

	if *over > 0 && written > 0 {
		os.Exit(1)
	}
}

func analyze(paths []string) []stat {
	stats := make([]stat, 0)
	for _, path := range paths {
		if isDir(path) {
			stats = analyzeDir(path, stats)
		} else {
			stats = analyzeFile(path, stats)
		}
	}
	return stats
}

func isDir(filename string) bool {
	fi, err := os.Stat(filename)
	return err == nil && fi.IsDir()
}

func analyzeFile(fname string, stats []stat) []stat {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fname, nil, 0)
	if err != nil {
		exitError(err)
	}
	return buildStats(f, fset, stats)
}

func analyzeDir(dirname string, stats []stat) []stat {
	files, _ := filepath.Glob(filepath.Join(dirname, "*.go"))
	for _, file := range files {
		stats = analyzeFile(file, stats)
	}
	return stats
}

func exitError(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func writeStats(w io.Writer, sortedStats []stat) int {
	for i, stat := range sortedStats {
		if i == *top {
			return i
		}
		if stat.Complexity <= *over {
			return i
		}
		fmt.Fprintln(w, stat)
	}
	return len(sortedStats)
}

func showAverage(stats []stat) {
	fmt.Printf("Average: %.3g\n", average(stats))
}

func average(stats []stat) float64 {
	total := 0
	for _, s := range stats {
		total += s.Complexity
	}
	return float64(total) / float64(len(stats))
}

type stat struct {
	PkgName    string
	FuncName   string
	Complexity int
	Pos        token.Position
}

func (s stat) String() string {
	return fmt.Sprintf("%d %s %s %s", s.Complexity, s.PkgName, s.FuncName, s.Pos)
}

type byComplexity []stat

func (s byComplexity) Len() int      { return len(s) }
func (s byComplexity) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byComplexity) Less(i, j int) bool {
	return s[i].Complexity >= s[j].Complexity
}

func buildStats(f *ast.File, fset *token.FileSet, stats []stat) []stat {
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			stats = append(stats, stat{
				PkgName:    f.Name.Name,
				FuncName:   funcName(fn),
				Complexity: complexity(fn),
				Pos:        fset.Position(fn.Pos()),
			})
		}
	}
	return stats
}

// funcName returns the name representation of a function or method:
// "(Type).Name" for methods or simply "Name" for functions.
func funcName(fn *ast.FuncDecl) string {
	if fn.Recv != nil {
		typ := fn.Recv.List[0].Type
		return fmt.Sprintf("(%s).%s", recvString(typ), fn.Name)
	}
	return fn.Name.Name
}

// recvString returns a string representation of recv of the
// form "T", "*T", or "BADRECV" (if not a proper receiver type).
func recvString(recv ast.Expr) string {
	switch t := recv.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + recvString(t.X)
	}
	return "BADRECV"
}

// complexity calculates the cyclomatic complexity of a function.
func complexity(fn *ast.FuncDecl) int {
	v := complexityVisitor{}
	ast.Walk(&v, fn)
	return v.Complexity
}

type complexityVisitor struct {
	// Complexity is the cyclomatic complexity
	Complexity int
}

// Visit implements the ast.Visitor interface.
func (v *complexityVisitor) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.FuncDecl, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
		v.Complexity++
	case *ast.BinaryExpr:
		if n.Op == token.LAND || n.Op == token.LOR {
			v.Complexity++
		}
	}
	return v
}
