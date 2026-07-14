// Package boringlint enforces a deliberately restricted Go dialect.
package boringlint

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// NoIterator rejects direct iter imports, iterator-shaped types in project type,
// function, and method declarations, iterator terms hidden by named constraints
// in project type declarations, and range-over-function.
var NoIterator = &analysis.Analyzer{
	Name: "noiterator",
	Doc: "reject range-over-function and iterator-shaped types in project type, function, and method declarations\n\n" +
		"noiterator reports direct iter imports, function-valued range operands, and " +
		"iterator-shaped constraints, fields, parameters, and results. Accept dependency " +
		"iterators without naming their type, materialize them at the call boundary, and " +
		"iterate concrete data. Project type declarations also cannot hide iterator-shaped " +
		"terms behind named constraints, including mixed unions and narrowed intersections.",
	URL:      "https://github.com/KrishRVH/boringlint#noiterator",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runNoIterator,
}

func runNoIterator(pass *analysis.Pass) (any, error) {
	inspection := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	inspection.Nodes(
		[]ast.Node{
			(*ast.ImportSpec)(nil),
			(*ast.FuncDecl)(nil),
			(*ast.TypeSpec)(nil),
			(*ast.RangeStmt)(nil),
		},
		func(node ast.Node, push bool) bool {
			if !push {
				return true
			}
			return inspectIteratorNode(pass, node)
		},
	)
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
		reportIteratorTypeSpec(pass, node)
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

func reportIteratorTypeSpec(pass *analysis.Pass, node *ast.TypeSpec) {
	if node.TypeParams == nil {
		if !reportIteratorTypes(pass, node.Type) {
			reportIteratorTypeTerms(pass, node.Type)
		}
		return
	}

	if !reportIteratorTypes(pass, node.TypeParams, node.Type) {
		reportIteratorTypeTerms(pass, node.TypeParams, node.Type)
	}
}

type iteratorConstraint struct {
	position   token.Pos
	typeParams []*types.TypeParam
}

type iteratorTypeReporter struct {
	pass               *analysis.Pass
	pendingConstraints []iteratorConstraint
	reportedTypeParams map[*types.TypeParam]bool
}

func reportIteratorTypes(pass *analysis.Pass, roots ...ast.Node) bool {
	reporter := iteratorTypeReporter{
		pass:               pass,
		reportedTypeParams: make(map[*types.TypeParam]bool),
	}
	reported := false
	for _, root := range roots {
		reported = reporter.walk(root) || reported
	}
	return reporter.reportPendingConstraints() || reported
}

func (reporter *iteratorTypeReporter) walk(root ast.Node) bool {
	reported := false
	ast.Inspect(root, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.Field:
			reported = reporter.walkField(node) || reported
			return false
		case ast.Expr:
			if reporter.reportExpression(node) {
				reported = true
				return false
			}
		}
		return true
	})
	return reported
}

func (reporter *iteratorTypeReporter) walkField(field *ast.Field) bool {
	if reporter.walk(field.Type) {
		return true
	}

	var constraint iteratorConstraint
	for _, name := range field.Names {
		object := reporter.pass.TypesInfo.Defs[name]
		if object == nil {
			continue
		}
		typeParam, ok := object.Type().(*types.TypeParam)
		if !ok || !isIteratorType(typeParam) {
			continue
		}
		if constraint.position == token.NoPos {
			constraint.position = name.Pos()
		}
		constraint.typeParams = append(constraint.typeParams, typeParam)
	}
	if len(constraint.typeParams) == 0 {
		return false
	}

	// Named interface constraints expose their iterator shape only through
	// go/types, so defer the fallback until the rest of the declaration is scanned.
	reporter.pendingConstraints = append(reporter.pendingConstraints, constraint)
	return false
}

func (reporter *iteratorTypeReporter) reportExpression(expr ast.Expr) bool {
	typ := reporter.pass.TypesInfo.TypeOf(expr)
	if !reportIteratorType(reporter.pass, expr.Pos(), typ) {
		return false
	}
	if typeParam, ok := types.Unalias(typ).(*types.TypeParam); ok {
		reporter.reportedTypeParams[typeParam] = true
	}
	return true
}

func (reporter *iteratorTypeReporter) reportPendingConstraints() bool {
	reported := false
	for _, constraint := range reporter.pendingConstraints {
		if reporter.typeParamWasReported(constraint.typeParams) {
			continue
		}
		reported = reportIteratorType(reporter.pass, constraint.position, constraint.typeParams[0]) || reported
	}
	return reported
}

func (reporter *iteratorTypeReporter) typeParamWasReported(typeParams []*types.TypeParam) bool {
	for _, typeParam := range typeParams {
		if reporter.reportedTypeParams[typeParam] {
			return true
		}
	}
	return false
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

func reportIteratorTypeTerms(pass *analysis.Pass, roots ...ast.Node) {
	for _, root := range roots {
		ast.Inspect(root, func(node ast.Node) bool {
			expr, ok := node.(ast.Expr)
			if !ok {
				return true
			}
			switch expr.(type) {
			case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
			default:
				return true
			}

			typ := pass.TypesInfo.TypeOf(expr)
			switch typ.(type) {
			case *types.Alias, *types.Named:
			default:
				return true
			}

			if !hasAcceptedSignature(typ, nil, isIteratorSignature, make(map[types.Type]bool)) {
				return true
			}

			pass.Reportf(
				expr.Pos(),
				"constraint %s contains an iterator-shaped term, which is forbidden by boringlint; materialize dependency iterators at the call boundary",
				types.TypeString(typ, types.RelativeTo(pass.Pkg)),
			)
			return false
		})
	}
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
	return ok && hasAcceptedSignature(
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
	return ok && hasAcceptedSignature(
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
	return ok && hasAcceptedSignature(
		typeParam.Constraint(),
		typeParam,
		func(*types.Signature) bool { return true },
		make(map[types.Type]bool),
	)
}

// Find an accepted signature term in typ. When typeParam is non-nil, let
// go/types also prove that every possible type is assignable to the candidate.
//
//nolint:gocognit // Constraint graphs require recursive handling of each go/types node kind.
func hasAcceptedSignature(
	typ types.Type,
	typeParam *types.TypeParam,
	accept func(*types.Signature) bool,
	seen map[types.Type]bool,
) bool {
	if typ == nil {
		return false
	}

	typ = types.Unalias(typ)
	if seen[typ] {
		return false
	}
	seen[typ] = true

	if signature, ok := typ.Underlying().(*types.Signature); ok {
		return accept(signature) &&
			(typeParam == nil || types.AssignableTo(typeParam, signature))
	}

	switch typ := typ.(type) {
	case *types.TypeParam:
		return hasAcceptedSignature(typ.Constraint(), typeParam, accept, seen)
	case *types.Named:
		return hasAcceptedSignature(typ.Underlying(), typeParam, accept, seen)
	case *types.Interface:
		for index := range typ.NumEmbeddeds() {
			if hasAcceptedSignature(typ.EmbeddedType(index), typeParam, accept, seen) {
				return true
			}
		}
	case *types.Union:
		for index := range typ.Len() {
			if hasAcceptedSignature(typ.Term(index).Type(), typeParam, accept, seen) {
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
		"nogenericmethod reports method declarations with method-local type parameters and " +
		"selector expressions that resolve to those methods. Methods using only receiver " +
		"type parameters remain allowed; use a package-level generic function instead.",
	URL:      "https://github.com/KrishRVH/boringlint#nogenericmethod",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      runNoGenericMethod,
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
	}

	inspection := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	inspection.Preorder([]ast.Node{(*ast.SelectorExpr)(nil)}, func(node ast.Node) {
		selector := node.(*ast.SelectorExpr)
		selection := pass.TypesInfo.Selections[selector]
		if selection == nil || !hasMethodTypeParameters(selection.Obj()) {
			return
		}

		pass.Reportf(
			selector.Sel.Pos(),
			"use of generic method %s is forbidden by boringlint; use a package-level generic function",
			selector.Sel.Name,
		)
	})
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
	signature := method.Signature()
	return signature.Recv() != nil &&
		signature.TypeParams().Len() > 0
}
