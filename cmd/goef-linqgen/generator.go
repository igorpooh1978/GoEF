package main

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
)

const marker = "//goef:query"

type declaration struct {
	source          *ast.FuncDecl
	entry           string
	rowType         string
	parameterName   string
	parameterType   string
	projectionType  string
	predicateField  string
	projectionField string
}

func generate(filename string, input []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, input, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	diagnostic := func(pos token.Pos, reason string) error {
		return fmt.Errorf("%s: %s", fset.PositionFor(pos, false), reason)
	}
	if len(file.Imports) != 0 {
		return nil, diagnostic(file.Imports[0].Pos(), "query source must be self-contained and contain no imports")
	}
	funcs := make(map[*ast.Comment]*ast.FuncDecl)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Doc != nil {
			for _, comment := range fn.Doc.List {
				funcs[comment] = fn
			}
		}
	}
	var declarations []declaration
	entries := make(map[string]bool)
	marked := make(map[*ast.FuncDecl]bool)
	for _, group := range file.Comments {
		for _, comment := range group.List {
			parts := strings.Fields(comment.Text)
			if len(parts) == 0 {
				continue
			}
			if parts[0] == "//go:build" || len(parts) >= 2 && parts[0] == "//" && parts[1] == "+build" {
				return nil, diagnostic(comment.Pos(), "query source must be available in every build")
			}
			if parts[0] != marker {
				continue
			}
			fn := funcs[comment]
			if fn == nil {
				return nil, diagnostic(comment.Pos(), "query marker must document a function")
			}
			if len(parts) != 2 || !token.IsIdentifier(parts[1]) || !ast.IsExported(parts[1]) {
				return nil, diagnostic(comment.Pos(), "query marker requires one exported entry point name")
			}
			if marked[fn] || entries[parts[1]] {
				return nil, diagnostic(comment.Pos(), "duplicate query marker or entry point name")
			}
			marked[fn], entries[parts[1]] = true, true
			declarations = append(declarations, declaration{source: fn, entry: parts[1]})
		}
	}
	if len(declarations) == 0 {
		return nil, diagnostic(file.Package, "expected at least one //goef:query function")
	}

	config := types.Config{GoVersion: "go1.27.4"}
	info := types.Info{Defs: make(map[*ast.Ident]types.Object), Selections: make(map[*ast.SelectorExpr]*types.Selection)}
	pkg, err := config.Check(file.Name.Name, fset, []*ast.File{file}, &info)
	if err != nil {
		return nil, err
	}
	for i := range declarations {
		d := &declarations[i]
		if pkg.Scope().Lookup(d.entry) != nil {
			return nil, diagnostic(d.source.Name.Pos(), "generated entry point already exists in the query source")
		}
		if err := d.bind(pkg, &info, diagnostic); err != nil {
			return nil, err
		}
	}
	return render(file.Name.Name, pkg.Scope(), declarations, sha256.Sum256(input))
}

func (d *declaration) bind(pkg *types.Package, info *types.Info, diagnostic func(token.Pos, string) error) error {
	fn := d.source
	signature := info.Defs[fn.Name].Type().(*types.Signature)
	params, results := signature.Params(), signature.Results()
	if fn.Recv != nil || fn.Type.TypeParams != nil || params.Len() != 2 || signature.Variadic() ||
		results.Len() != 2 || !types.Identical(results.At(0).Type(), types.Typ[types.Bool]) {
		return diagnostic(fn.Type.Pos(), "expected func(row Row, argument Scalar) (bool, Scalar)")
	}

	row, param := params.At(0), params.At(1)
	rowType, named := types.Unalias(row.Type()).(*types.Named)
	if !named {
		return diagnostic(row.Pos(), "row must be a named struct in the query source")
	}
	_, structure := rowType.Underlying().(*types.Struct)
	if !structure || rowType.Obj().Pkg() != pkg || rowType.TypeArgs().Len() != 0 {
		return diagnostic(row.Pos(), "row must be a named non-generic struct in the query source")
	}
	if !scalar(param.Type()) || !scalar(results.At(1).Type()) {
		return diagnostic(fn.Type.Pos(), "argument and projection must be supported scalar types")
	}
	if row.Name() == "" || row.Name() == "_" || param.Name() == "" || param.Name() == "_" {
		return diagnostic(fn.Type.Pos(), "row and argument must have names")
	}
	if fn.Body == nil || len(fn.Body.List) != 1 {
		return diagnostic(fn.Type.Pos(), "query body must contain one return statement")
	}
	result, ok := fn.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(result.Results) != 2 {
		return diagnostic(fn.Body.List[0].Pos(), "query must return a predicate and a projected row field")
	}

	predicate, ok := ast.Unparen(result.Results[0]).(*ast.BinaryExpr)
	if !ok || predicate.Op != token.EQL {
		return diagnostic(result.Results[0].Pos(), "predicate must compare a direct row field to the argument with ==")
	}
	selected := directField(info, predicate.X, row.Name())
	argument, matches := ast.Unparen(predicate.Y).(*ast.Ident)
	if selected == nil || !matches || argument.Name != param.Name() {
		return diagnostic(predicate.Pos(), "expected row.Field == argument")
	}
	if !types.Identical(selected.Type(), param.Type()) {
		return diagnostic(predicate.Pos(), "compared field and argument must have identical types")
	}
	projected := directField(info, result.Results[1], row.Name())
	if projected == nil || !types.Identical(projected.Type(), results.At(1).Type()) {
		return diagnostic(result.Results[1].Pos(), "projection must be a direct row field of the exact result type")
	}

	d.rowType = types.TypeString(row.Type(), func(*types.Package) string { return "" })
	d.parameterName = param.Name()
	d.parameterType = types.TypeString(param.Type(), func(*types.Package) string { return "" })
	d.projectionType = types.TypeString(results.At(1).Type(), func(*types.Package) string { return "" })
	d.predicateField = selected.Name()
	d.projectionField = projected.Name()
	return nil
}

func directField(info *types.Info, input ast.Expr, row string) *types.Var {
	field, ok := ast.Unparen(input).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	base, ok := ast.Unparen(field.X).(*ast.Ident)
	if !ok || base.Name != row {
		return nil
	}
	selections := info.Selections[field]
	if selections == nil || selections.Kind() != types.FieldVal || len(selections.Index()) != 1 {
		return nil
	}
	return selections.Obj().(*types.Var)
}

func scalar(t types.Type) bool {
	basic, ok := t.Underlying().(*types.Basic)
	if !ok {
		return false
	}
	switch basic.Kind() {
	case types.Bool, types.Int, types.Int8, types.Int16, types.Int32, types.Int64, types.String:
		return true
	}
	return false
}

func render(packageName string, scope *types.Scope, declarations []declaration, fingerprint [sha256.Size]byte) ([]byte, error) {
	available := func(name string) string {
		for scope.Lookup(name) != nil {
			name += "_"
		}
		return name
	}
	bindings, value, row := available("goefbindings"), available("goefValue"), available("goefRow")

	var source strings.Builder
	source.WriteString(generatedHeader)
	fmt.Fprintf(&source, "// Source SHA-256: %x\n\npackage %s\n\n", fingerprint, packageName)
	fmt.Fprintf(&source, "import %s %q\n\n", bindings, "github.com/igorpooh1978/GoEF/linq/bindings")
	for _, d := range declarations {
		fmt.Fprintf(&source, "// %s binds %s for this invocation.\n", d.entry, d.parameterName)
		fmt.Fprintf(&source, "func %s(%s %s) %s.Equality[%s, %s, %s] {\n",
			d.entry, value, d.parameterType, bindings, d.rowType, d.parameterType, d.projectionType)
		fmt.Fprintf(&source, "return %s.BindEquality(\n", bindings)
		fmt.Fprintf(&source, "%s.EqualityShape{Field: %q, Parameter: %q, Projection: %q},\n", bindings, d.predicateField, d.parameterName, d.projectionField)
		fmt.Fprintf(&source, "%s,\nfunc(%s %s) %s { return %s.%s },\n)\n}\n\n", value, row, d.rowType, d.projectionType, row, d.projectionField)
	}
	return format.Source([]byte(source.String()))
}
