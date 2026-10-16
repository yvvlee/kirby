// Command stylecheck enforces the local rules that are not available in the
// public golangci-lint distribution. It deliberately skips generated and test
// files, matching the production-code scope of the reference lint policy.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type diagnostic struct {
	position token.Position
	rule     string
	message  string
}

type checker struct {
	files       *token.FileSet
	diagnostics []diagnostic
}

func main() {
	check := &checker{files: token.NewFileSet()}
	if err := check.directory("."); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for _, item := range check.diagnostics {
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", item.position, item.rule, item.message)
	}
	if len(check.diagnostics) > 0 {
		os.Exit(1)
	}
}

func (c *checker) directory(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "testdata") {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		return c.file(path, nil)
	})
}

func (c *checker) file(path string, source any) error {
	file, err := parser.ParseFile(c.files, path, source, parser.ParseComments)
	if err != nil {
		return err
	}
	if ast.IsGenerated(file) {
		return nil
	}
	ast.Inspect(file, c.node)
	return nil
}

func (c *checker) add(node ast.Node, rule, message string) {
	c.diagnostics = append(c.diagnostics, diagnostic{
		position: c.files.Position(node.Pos()),
		rule:     rule,
		message:  message,
	})
}

func (c *checker) node(node ast.Node) bool {
	switch value := node.(type) {
	case *ast.FuncDecl:
		c.function(value)
	case *ast.InterfaceType:
		c.interfaceMethods(value)
	case *ast.CompositeLit:
		c.fields(value)
	case *ast.StructType:
		c.structFields(value)
	case *ast.CallExpr:
		c.logCall(value)
	case *ast.Ident:
		if value.Name == "iota" {
			c.add(value, "noiota", "use explicit constant values")
		}
	}
	return true
}

func (c *checker) function(fn *ast.FuncDecl) {
	// Dependency injection constructors intentionally enumerate dependencies.
	if !strings.HasPrefix(fn.Name.Name, "New") && parameterCount(fn.Type.Params) > 5 {
		c.add(fn, "argscount", "group operation inputs instead of passing more than five parameters")
	}
	if fn.Body != nil {
		visitor := nesting{
			check:  c,
			elseIf: make(map[*ast.IfStmt]bool),
		}
		ast.Walk(visitor, fn.Body)
	}
	if !fn.Name.IsExported() {
		return
	}
	doc := fn.Doc.Text()
	if !strings.HasPrefix(doc, fn.Name.Name+" ") || !strings.Contains(doc, "Parameters:") || !strings.Contains(doc, "Returns:") || !strings.Contains(doc, "Throws:") {
		c.add(fn, "docstring", "describe the operation and include Parameters, Returns, and Throws")
	}
}

func parameterCount(parameters *ast.FieldList) int {
	count := 0
	for _, field := range parameters.List {
		count += max(1, len(field.Names))
	}
	return count
}

func (c *checker) interfaceMethods(value *ast.InterfaceType) {
	for _, method := range value.Methods.List {
		fn, ok := method.Type.(*ast.FuncType)
		if !ok {
			continue
		}
		c.interfaceParameters(fn)
	}
}

func (c *checker) interfaceParameters(fn *ast.FuncType) {
	for _, parameter := range fn.Params.List {
		if len(parameter.Names) == 0 {
			c.add(parameter, "namedparams", "name interface parameters so adjacent values have a clear meaning")
		}
	}
}

func (c *checker) fields(value *ast.CompositeLit) {
	if len(value.Elts) < 2 {
		return
	}
	previous := -1
	for _, element := range value.Elts {
		if _, ok := element.(*ast.KeyValueExpr); !ok {
			continue
		}
		line := c.files.Position(element.Pos()).Line
		if line == previous {
			c.add(element, "fieldlines", "put each keyed field on its own line")
		}
		previous = line
	}
}

func (c *checker) structFields(value *ast.StructType) {
	for _, field := range value.Fields.List {
		if len(field.Names) > 1 {
			c.add(field, "fieldlines", "declare each struct field separately")
		}
	}
}

func (c *checker) logCall(call *ast.CallExpr) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || call.Ellipsis.IsValid() {
		return
	}
	switch selector.Sel.Name {
	case "Debugw", "Infow", "Warnw", "Errorw", "Fatalw":
		if len(call.Args)%2 != 0 {
			c.add(call, "mustlogpair", "structured log fields must be key/value pairs")
		}
	case "Log":
		if len(call.Args) > 0 && (len(call.Args)-1)%2 != 0 {
			c.add(call, "mustlogpair", "log fields after the level must be key/value pairs")
		}
	}
}

type nesting struct {
	check  *checker
	depth  int
	elseIf map[*ast.IfStmt]bool
}

// Visit counts control structures, treating an else-if chain as one level.
// Parameters: node is the syntax node supplied by ast.Walk.
// Returns: a value visitor carrying the depth for this node's children.
// Throws: none.
func (v nesting) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}
	control := false
	switch value := node.(type) {
	case *ast.IfStmt:
		if next, ok := value.Else.(*ast.IfStmt); ok {
			v.elseIf[next] = true
		}
		control = !v.elseIf[value]
	case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		control = true
	}
	if control {
		v.depth++
	}
	if v.depth == 3 && control {
		v.check.add(node, "nesting", "extract a business step to keep control flow at two levels")
	}
	return v
}
