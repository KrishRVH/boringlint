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
	inspection.Preorder(
		[]ast.Node{
			(*ast.ImportSpec)(nil),
			(*ast.FuncDecl)(nil),
			(*ast.TypeSpec)(nil),
			(*ast.RangeStmt)(nil),
		},
		func(node ast.Node) {
			inspectIteratorNode(pass, node)
		},
	)
	//nolint:nilnil // analysis.Analyzer uses a nil result to mean no exported fact.
	return nil, nil
}

func inspectIteratorNode(pass *analysis.Pass, node ast.Node) {
	switch node := node.(type) {
	case *ast.ImportSpec:
		path, err := strconv.Unquote(node.Path.Value)
		if err == nil && path == "iter" {
			pass.Reportf(
				node.Path.Pos(),
				"import of iter is forbidden by boringlint; accept dependency iterators without naming their type and materialize them at the boundary",
			)
		}
	case *ast.FuncDecl:
		if object := pass.TypesInfo.Defs[node.Name]; object != nil {
			reportIteratorType(pass, node.Name.Pos(), object.Type())
		}
		newIteratorTypeReporter(pass, false).report(node.Type)
	case *ast.TypeSpec:
		reportIteratorTypeSpec(pass, node)
	case *ast.RangeStmt:
		typ := pass.TypesInfo.TypeOf(node.X)
		if !isRangeFunctionType(typ) {
			return
		}

		pass.Reportf(
			node.Range,
			"range over a function value (%s) is forbidden by boringlint; iterate concrete data or materialize at the dependency boundary",
			types.TypeString(typ, types.RelativeTo(pass.Pkg)),
		)
	}
}

func reportIteratorTypeSpec(pass *analysis.Pass, node *ast.TypeSpec) {
	reporter := newIteratorTypeReporter(pass, true)
	// A nil *ast.FieldList is a non-nil ast.Node, so include it only when present.
	roots := []ast.Node{node.Type}
	if node.TypeParams != nil {
		reporter.markIteratorTypeParams(node.TypeParams)
		roots = []ast.Node{node.TypeParams, node.Type}
	}
	covered := reporter.report(roots...)
	reportIteratorTypeTerms(pass, covered, roots...)
}

type iteratorConstraint struct {
	position   token.Pos
	typeParams []*types.TypeParam
}

type iteratorTypeReporter struct {
	pass               *analysis.Pass
	pendingConstraints []iteratorConstraint
	reportedTypeParams map[*types.TypeParam]bool
	coveredTypeTerms   map[ast.Expr]bool
	// preferTypeTerms selects type-declaration reporting: hidden constraint
	// terms and type parameters are reported at constraints, not at uses.
	preferTypeTerms bool
}

func newIteratorTypeReporter(pass *analysis.Pass, preferTypeTerms bool) *iteratorTypeReporter {
	return &iteratorTypeReporter{
		pass:               pass,
		reportedTypeParams: make(map[*types.TypeParam]bool),
		coveredTypeTerms:   make(map[ast.Expr]bool),
		preferTypeTerms:    preferTypeTerms,
	}
}

// markIteratorTypeParams records a type declaration's iterator-shaped type
// parameters before any root is scanned. Each is reported at its own
// constraint, so references from other constraints, including earlier ones,
// are not reported again.
func (reporter *iteratorTypeReporter) markIteratorTypeParams(typeParams *ast.FieldList) {
	for _, field := range typeParams.List {
		for _, name := range field.Names {
			if typeParam := reporter.declaredTypeParam(name); typeParam != nil && isIteratorType(typeParam) {
				reporter.reportedTypeParams[typeParam] = true
			}
		}
	}
}

// report scans every root before resolving pending constraints and returns
// expressions already covered by either diagnostic path.
func (reporter *iteratorTypeReporter) report(roots ...ast.Node) map[ast.Expr]bool {
	for _, root := range roots {
		reporter.walk(root)
	}
	reporter.reportPendingConstraints()
	return reporter.coveredTypeTerms
}

func (reporter *iteratorTypeReporter) walk(root ast.Node) bool {
	reported := false
	ast.Inspect(root, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.ArrayType:
			// Length expressions may contain function literals and local variables.
			// The outer inspector handles nested declarations and range statements.
			reported = reporter.walk(node.Elt) || reported
			return false
		case *ast.Field:
			reported = reporter.walkField(node) || reported
			return false
		case ast.Expr:
			reported = reporter.reportExpression(node) || reported
		}
		return true
	})
	return reported
}

func (reporter *iteratorTypeReporter) walkField(field *ast.Field) bool {
	reported := reporter.walk(field.Type)
	if reporter.preferTypeTerms {
		reported = reportIteratorTypeTerms(
			reporter.pass,
			reporter.coveredTypeTerms,
			field.Type,
		) || reported
	}

	var constraint iteratorConstraint
	for _, name := range field.Names {
		reporter.collectConstraintTypeParam(&constraint, name)
	}
	if reported || len(constraint.typeParams) == 0 {
		return reported
	}

	// A named constraint exposes its iterator shape only through go/types. Defer
	// the fallback until the rest of the declaration is scanned, so a reported
	// use can stand in for it.
	reporter.pendingConstraints = append(reporter.pendingConstraints, constraint)
	return false
}

func (reporter *iteratorTypeReporter) declaredTypeParam(name *ast.Ident) *types.TypeParam {
	typeName, ok := reporter.pass.TypesInfo.Defs[name].(*types.TypeName)
	if !ok {
		return nil
	}
	typeParam, _ := typeName.Type().(*types.TypeParam)
	return typeParam
}

func (reporter *iteratorTypeReporter) collectConstraintTypeParam(
	constraint *iteratorConstraint,
	name *ast.Ident,
) {
	typeParam := reporter.declaredTypeParam(name)
	if typeParam == nil || !isIteratorType(typeParam) {
		return
	}
	if constraint.position == token.NoPos {
		constraint.position = name.Pos()
	}
	constraint.typeParams = append(constraint.typeParams, typeParam)
}

func (reporter *iteratorTypeReporter) reportExpression(expr ast.Expr) bool {
	if reporter.coveredTypeTerms[expr] {
		return false
	}
	typ := reporter.pass.TypesInfo.TypeOf(expr)
	typeParam, isTypeParam := types.Unalias(typ).(*types.TypeParam)
	if isTypeParam && reporter.preferTypeTerms && reporter.reportedTypeParams[typeParam] {
		return false
	}
	if !reportIteratorType(reporter.pass, expr.Pos(), typ) {
		return false
	}
	if isTypeParam {
		reporter.reportedTypeParams[typeParam] = true
	}
	reporter.coveredTypeTerms[expr] = true
	coverEquivalentTypeExpressions(reporter.coveredTypeTerms, expr)
	return true
}

func (reporter *iteratorTypeReporter) reportPendingConstraints() {
	for _, constraint := range reporter.pendingConstraints {
		// Type declarations mark every iterator type parameter up front, so only
		// function declarations can rely on a use having been reported.
		if !reporter.preferTypeTerms && reporter.anyTypeParamReported(constraint.typeParams) {
			continue
		}
		reportIteratorType(reporter.pass, constraint.position, constraint.typeParams[0])
	}
}

// anyTypeParamReported relies on go/types type parameters being identical only
// to themselves, so pointer lookup matches types.Identical.
func (reporter *iteratorTypeReporter) anyTypeParamReported(typeParams []*types.TypeParam) bool {
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

func reportIteratorTypeTerms(pass *analysis.Pass, covered map[ast.Expr]bool, roots ...ast.Node) bool {
	reported := false
	for _, root := range roots {
		ast.Inspect(root, func(node ast.Node) bool {
			if array, ok := node.(*ast.ArrayType); ok {
				reported = reportIteratorTypeTerms(pass, covered, array.Elt) || reported
				return false
			}
			expr, ok := node.(ast.Expr)
			if !ok {
				return true
			}
			if covered[expr] {
				return true
			}
			typ := iteratorTermType(pass, expr)
			if typ == nil {
				return true
			}

			pass.Reportf(
				expr.Pos(),
				"constraint %s contains an iterator-shaped term, which is forbidden by boringlint; materialize dependency iterators at the call boundary",
				types.TypeString(typ, types.RelativeTo(pass.Pkg)),
			)
			covered[expr] = true
			coverEquivalentTypeExpressions(covered, expr)
			reported = true
			return true
		})
	}
	return reported
}

func coverEquivalentTypeExpressions(covered map[ast.Expr]bool, expr ast.Expr) {
	for {
		var next ast.Expr
		switch expr := expr.(type) {
		case *ast.ParenExpr:
			next = expr.X
		case *ast.IndexExpr:
			next = expr.X
		case *ast.IndexListExpr:
			next = expr.X
		case *ast.SelectorExpr:
			next = expr.Sel
		}
		if next == nil {
			return
		}
		covered[next] = true
		expr = next
	}
}

func iteratorTermType(pass *analysis.Pass, expr ast.Expr) types.Type {
	if referencedTypeName(pass, expr) == nil {
		return nil
	}

	typ := pass.TypesInfo.TypeOf(expr)
	switch typ.(type) {
	case *types.Alias, *types.Named:
	default:
		return nil
	}

	if !hasAcceptedSignature(typ, nil, isIteratorSignature, make(map[types.Type]bool)) {
		return nil
	}
	return typ
}

func referencedTypeName(pass *analysis.Pass, expr ast.Expr) *types.TypeName {
	switch expr := expr.(type) {
	case *ast.Ident:
		typeName, _ := pass.TypesInfo.Uses[expr].(*types.TypeName)
		return typeName
	case *ast.SelectorExpr:
		typeName, _ := pass.TypesInfo.Uses[expr.Sel].(*types.TypeName)
		return typeName
	case *ast.IndexExpr:
		return referencedTypeName(pass, expr.X)
	case *ast.IndexListExpr:
		return referencedTypeName(pass, expr.X)
	default:
		return nil
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
	return ok && hasOnlyAcceptedSignatures(typeParam, isIteratorSignature)
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
	return ok && hasOnlyAcceptedSignatures(typeParam, isYieldSignature)
}

func isYieldSignature(signature *types.Signature) bool {
	return !signature.Variadic() &&
		signature.Params().Len() <= 2 &&
		signature.Results().Len() == 1 &&
		isBooleanType(signature.Results().At(0).Type())
}

func isBooleanType(typ types.Type) bool {
	typ = types.Unalias(typ)
	if types.Identical(typ, types.Typ[types.Bool]) {
		return true
	}

	typeParam, ok := typ.(*types.TypeParam)
	return ok && hasOnlyAcceptedTypes(typeParam, func(candidate types.Type) bool {
		return types.Identical(candidate, types.Typ[types.Bool])
	})
}

// isRangeFunctionType reports whether typ is a function range operand. go/types
// validates the signature before the analyzer runs, so this only distinguishes
// function ranges from the other legal range operands.
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

func hasOnlyAcceptedSignatures(
	typeParam *types.TypeParam,
	accept func(*types.Signature) bool,
) bool {
	return hasOnlyAcceptedTypes(typeParam, func(candidate types.Type) bool {
		signature, ok := candidate.Underlying().(*types.Signature)
		return ok && accept(signature)
	})
}

// hasOnlyAcceptedTypes reports whether typeParam's type set has at least one
// finite candidate and accept holds for every candidate.
func hasOnlyAcceptedTypes(
	typeParam *types.TypeParam,
	accept func(types.Type) bool,
) bool {
	constraint, ok := typeParam.Constraint().Underlying().(*types.Interface)
	if !ok {
		return false
	}

	// Exact terms are candidates as written. A ~T term whose underlying type
	// can have methods becomes a named witness carrying the constraint's
	// methods; other ~T terms stay direct candidates. Satisfies then filters
	// methods, comparability, and intersections, and a type set with no
	// surviving candidate does not match.
	var terms []*types.Term
	collectTypeSetTerms(typeParam.Constraint(), make(map[types.Type]bool), &terms)
	found := false
	for _, term := range terms {
		candidate := types.Unalias(term.Type())
		if term.Tilde() && canDeclareMethods(candidate.Underlying()) {
			candidate = newMethodWitness(typeParam, candidate.Underlying(), constraint)
		}
		if candidate == nil || !types.Satisfies(candidate, constraint) {
			continue
		}
		found = true
		if !accept(candidate) {
			return false
		}
	}
	return found
}

func collectTypeSetTerms(
	typ types.Type,
	seen map[types.Type]bool,
	terms *[]*types.Term,
) {
	typ = types.Unalias(typ)
	if seen[typ] {
		return
	}
	seen[typ] = true

	switch typ := typ.(type) {
	case *types.Named:
		if constraint, ok := typ.Underlying().(*types.Interface); ok {
			collectTypeSetTerms(constraint, seen, terms)
			return
		}
		appendTypeSetTerm(types.NewTerm(false, typ), terms)
	case *types.Interface:
		for index := range typ.NumEmbeddeds() {
			collectTypeSetTerms(typ.EmbeddedType(index), seen, terms)
		}
	case *types.Union:
		collectUnionTypeSetTerms(typ, seen, terms)
	default:
		appendTypeSetTerm(types.NewTerm(false, typ), terms)
	}
}

func collectUnionTypeSetTerms(
	union *types.Union,
	seen map[types.Type]bool,
	terms *[]*types.Term,
) {
	// The wrapper is Empty only when the union denotes the all-types set, which
	// has no finite leaf candidates to prove universally accepted.
	if types.NewInterfaceType(nil, []types.Type{union}).Complete().Empty() {
		return
	}
	for index := range union.Len() {
		term := union.Term(index)
		termType := types.Unalias(term.Type())
		if _, ok := termType.Underlying().(*types.Interface); ok && !term.Tilde() {
			collectTypeSetTerms(termType, seen, terms)
			continue
		}
		appendTypeSetTerm(types.NewTerm(term.Tilde(), termType), terms)
	}
}

func appendTypeSetTerm(
	term *types.Term,
	terms *[]*types.Term,
) {
	for _, existing := range *terms {
		if existing.Tilde() == term.Tilde() && types.Identical(existing.Type(), term.Type()) {
			return
		}
	}
	*terms = append(*terms, term)
}

func canDeclareMethods(underlying types.Type) bool {
	switch underlying := underlying.(type) {
	case *types.Array, *types.Chan, *types.Map, *types.Signature, *types.Slice, *types.Struct:
		return true
	case *types.Basic:
		// unsafe.Pointer cannot be a receiver base type.
		return underlying.Kind() != types.UnsafePointer
	default:
		return false
	}
}

// newMethodWitness returns a named type with the given underlying type and the
// constraint's methods, or nil when no Go declaration could have that shape.
func newMethodWitness(
	typeParam *types.TypeParam,
	underlying types.Type,
	constraint *types.Interface,
) types.Type {
	pkg := witnessPackage(typeParam, constraint)
	typeName := types.NewTypeName(token.NoPos, pkg, "boringlintWitness", nil)
	witness := types.NewNamed(typeName, underlying, nil)
	for index := range constraint.NumMethods() {
		method := constraint.Method(index)
		signature := method.Signature()
		receiver := types.NewVar(token.NoPos, pkg, "", witness)
		witness.AddMethod(types.NewFunc(
			method.Pos(),
			pkg,
			method.Name(),
			types.NewSignatureType(
				receiver,
				nil,
				nil,
				signature.Params(),
				signature.Results(),
				signature.Variadic(),
			),
		))
	}

	return validatedMethodWitness(witness, constraint)
}

// witnessPackage chooses the package that declares a method witness: the
// package of the constraint's first unexported method, since only that package
// can declare it, or else the type parameter's package.
func witnessPackage(typeParam *types.TypeParam, constraint *types.Interface) *types.Package {
	for index := range constraint.NumMethods() {
		if method := constraint.Method(index); !method.Exported() {
			return method.Pkg()
		}
	}
	return typeParam.Obj().Pkg()
}

func validatedMethodWitness(witness *types.Named, constraint *types.Interface) types.Type {
	// Named.AddMethod permits synthetic field/method collisions that Go source
	// does not, and the witness package cannot declare another package's
	// unexported methods. The computed method set filters both.
	methodSet := types.NewMethodSet(witness)
	for index := range constraint.NumMethods() {
		method := constraint.Method(index)
		if methodSet.Lookup(method.Pkg(), method.Name()) == nil {
			return nil
		}
	}
	return witness
}

// hasAcceptedSignature reports whether typ contains an accepted signature term.
// When typeParam is non-nil, go/types must also prove that every type in its
// type set is assignable to that signature.
//
//nolint:gocognit // Constraint graphs require recursive handling of each go/types node kind.
func hasAcceptedSignature(
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
		return accept(signature) &&
			(typeParam == nil || types.AssignableTo(typeParam, signature))
	}

	switch typ := typ.(type) {
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
	inspection := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	inspection.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.SelectorExpr)(nil)}, func(node ast.Node) {
		switch node := node.(type) {
		case *ast.FuncDecl:
			if node.Recv == nil || node.Type.TypeParams.NumFields() == 0 {
				return
			}
			pass.Reportf(
				node.Name.Pos(),
				"generic method %s declares method-local type parameters, which are forbidden by boringlint; use a package-level generic function",
				node.Name.Name,
			)
		case *ast.SelectorExpr:
			selection := pass.TypesInfo.Selections[node]
			if selection == nil || !hasMethodTypeParameters(selection.Obj()) {
				return
			}
			pass.Reportf(
				node.Sel.Pos(),
				"use of generic method %s is forbidden by boringlint; use a package-level generic function",
				node.Sel.Name,
			)
		}
	})
	//nolint:nilnil // analysis.Analyzer uses a nil result to mean no exported fact.
	return nil, nil
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
