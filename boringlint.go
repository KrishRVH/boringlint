// Package boringlint enforces a deliberately restricted Go dialect.
package boringlint

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
)

// NoIterator rejects direct iter imports, iterator-shaped types in project type,
// function, and method declarations, and range-over-function.
var NoIterator = &analysis.Analyzer{
	Name: "noiterator",
	Doc: "reject range-over-function and iterator-shaped types in project type, function, and method declarations\n\n" +
		"noiterator rejects direct iter imports, range-over-function, and " +
		"iterator-shaped types in project type, function, and method declarations",
	URL: "https://github.com/KrishRVH/boringlint#noiterator",
	Run: runNoIterator,
}

func runNoIterator(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			return inspectIteratorNode(pass, node)
		})
	}
	//nolint:nilnil // analysis.Analyzer uses a nil result to mean no exported fact.
	return nil, nil
}

func inspectIteratorNode(pass *analysis.Pass, node ast.Node) bool {
	switch node := node.(type) {
	case *ast.ImportSpec:
		path, err := strconv.Unquote(node.Path.Value)
		if err == nil && path == "iter" {
			pass.Reportf(
				node.Path.Pos(),
				"import of iter is forbidden by boringlint; accept dependency iterators without naming their type and materialize them at the boundary",
			)
		}
		return false
	case *ast.FuncDecl:
		if object := pass.TypesInfo.Defs[node.Name]; object != nil {
			reportIteratorType(pass, node.Name.Pos(), object.Type())
		}
		reportIteratorTypes(pass, node.Type)
	case *ast.TypeSpec:
		if node.TypeParams != nil {
			reportIteratorTypes(pass, node.TypeParams)
		}
		reportIteratorTypes(pass, node.Type)
		return false
	case *ast.RangeStmt:
		typ := pass.TypesInfo.TypeOf(node.X)
		if !isRangeFunctionType(typ) {
			return true
		}

		pass.Reportf(
			node.Range,
			"range over a function value (%s) is forbidden by boringlint; iterate concrete data or materialize at the dependency boundary",
			types.TypeString(typ, types.RelativeTo(pass.Pkg)),
		)
	}
	return true
}

func reportIteratorTypes(pass *analysis.Pass, root ast.Node) {
	ast.Inspect(root, func(node ast.Node) bool {
		if field, ok := node.(*ast.Field); ok {
			reportIteratorTypes(pass, field.Type)
			return false
		}

		expr, ok := node.(ast.Expr)
		if !ok {
			return true
		}
		typ := pass.TypesInfo.TypeOf(expr)
		return !reportIteratorType(pass, expr.Pos(), typ)
	})
}

func reportIteratorType(pass *analysis.Pass, position token.Pos, typ types.Type) bool {
	if !isIteratorType(typ) {
		return false
	}

	pass.Reportf(
		position,
		"iterator-shaped type %s is forbidden by boringlint; materialize dependency iterators at the call boundary",
		types.TypeString(typ, types.RelativeTo(pass.Pkg)),
	)
	return true
}

func isIteratorType(typ types.Type) bool {
	if typ == nil {
		return false
	}

	typ = types.Unalias(typ)
	if signature, ok := typ.Underlying().(*types.Signature); ok {
		return isIteratorSignature(signature)
	}

	typeParam, ok := typ.(*types.TypeParam)
	return ok && hasAssignableSignature(
		typeParam.Constraint(),
		typeParam,
		isIteratorSignature,
		make(map[types.Type]bool),
	)
}

func isIteratorSignature(signature *types.Signature) bool {
	if signature.Params().Len() != 1 || signature.Results().Len() != 0 {
		return false
	}

	yieldType := types.Unalias(signature.Params().At(0).Type())
	if yield, ok := yieldType.Underlying().(*types.Signature); ok {
		return isYieldSignature(yield)
	}

	typeParam, ok := yieldType.(*types.TypeParam)
	return ok && hasAssignableSignature(
		typeParam.Constraint(),
		typeParam,
		isYieldSignature,
		make(map[types.Type]bool),
	)
}

func isYieldSignature(signature *types.Signature) bool {
	return !signature.Variadic() &&
		signature.Params().Len() <= 2 &&
		signature.Results().Len() == 1 &&
		types.Identical(signature.Results().At(0).Type(), types.Typ[types.Bool])
}

// go/types validates the signature before the analyzer runs; this only
// distinguishes function ranges from the other legal range operands.
func isRangeFunctionType(typ types.Type) bool {
	if typ == nil {
		return false
	}

	typ = types.Unalias(typ)
	if _, ok := typ.Underlying().(*types.Signature); ok {
		return true
	}

	typeParam, ok := typ.(*types.TypeParam)
	return ok && hasAssignableSignature(
		typeParam.Constraint(),
		typeParam,
		func(*types.Signature) bool { return true },
		make(map[types.Type]bool),
	)
}

// An iterator-shaped type parameter has a common underlying signature. Find a
// candidate in the constraint, then let go/types prove that every possible type
// is assignable to it.
//
//nolint:gocognit // Constraint graphs require recursive handling of each go/types node kind.
func hasAssignableSignature(
	typ types.Type,
	typeParam *types.TypeParam,
	accept func(*types.Signature) bool,
	seen map[types.Type]bool,
) bool {
	typ = types.Unalias(typ)
	if seen[typ] {
		return false
	}
	seen[typ] = true

	if signature, ok := typ.Underlying().(*types.Signature); ok {
		return accept(signature) && types.AssignableTo(typeParam, signature)
	}

	switch typ := typ.(type) {
	case *types.TypeParam:
		return hasAssignableSignature(typ.Constraint(), typeParam, accept, seen)
	case *types.Named:
		return hasAssignableSignature(typ.Underlying(), typeParam, accept, seen)
	case *types.Interface:
		for index := range typ.NumEmbeddeds() {
			if hasAssignableSignature(typ.EmbeddedType(index), typeParam, accept, seen) {
				return true
			}
		}
	case *types.Union:
		for index := range typ.Len() {
			if hasAssignableSignature(typ.Term(index).Type(), typeParam, accept, seen) {
				return true
			}
		}
	}
	return false
}

// NoGenericMethod rejects generic method declarations and uses.
var NoGenericMethod = &analysis.Analyzer{
	Name: "nogenericmethod",
	Doc: "reject generic method declarations and uses\n\n" +
		"nogenericmethod rejects method-local type parameters introduced in Go 1.27; " +
		"use a package-level generic function",
	URL: "https://github.com/KrishRVH/boringlint#nogenericmethod",
	Run: runNoGenericMethod,
}

func runNoGenericMethod(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		reportGenericMethods(file, func(decl *ast.FuncDecl) {
			pass.Reportf(
				decl.Name.Pos(),
				"generic method %s declares method-local type parameters, which are forbidden by boringlint; use a package-level generic function",
				decl.Name.Name,
			)
		})

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			selection := pass.TypesInfo.Selections[selector]
			if selection == nil || !hasMethodTypeParameters(selection.Obj()) {
				return true
			}

			pass.Reportf(
				selector.Sel.Pos(),
				"use of generic method %s is forbidden by boringlint; use a package-level generic function",
				selector.Sel.Name,
			)
			return true
		})
	}
	//nolint:nilnil // analysis.Analyzer uses a nil result to mean no exported fact.
	return nil, nil
}

// reportGenericMethods calls report for each method declaration with its own
// type parameter list. It remains directly testable on toolchains where that
// syntax is still rejected before an analysis driver can run.
func reportGenericMethods(file *ast.File, report func(*ast.FuncDecl)) {
	for _, declaration := range file.Decls {
		decl, ok := declaration.(*ast.FuncDecl)
		if !ok || decl.Recv == nil || decl.Type == nil {
			continue
		}
		if params := decl.Type.TypeParams; params != nil && len(params.List) > 0 {
			report(decl)
		}
	}
}

func hasMethodTypeParameters(object types.Object) bool {
	method, ok := object.(*types.Func)
	if !ok {
		return false
	}
	signature, ok := method.Type().(*types.Signature)
	return ok && signature.Recv() != nil &&
		signature.TypeParams() != nil && signature.TypeParams().Len() > 0
}
