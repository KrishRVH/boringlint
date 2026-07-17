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
		reportIteratorTypes(pass, false, node.Type)
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
		covered := reportIteratorTypes(pass, true, node.Type)
		reportIteratorTypeTerms(pass, covered, node.Type)
		return
	}

	covered := reportIteratorTypes(pass, true, node.TypeParams, node.Type)
	reportIteratorTypeTerms(pass, covered, node.TypeParams, node.Type)
}

type iteratorConstraint struct {
	position   token.Pos
	root       ast.Expr
	typeParams []*types.TypeParam
}

type iteratorTypeReporter struct {
	pass               *analysis.Pass
	pendingConstraints []iteratorConstraint
	reportedTypeParams map[*types.TypeParam]bool
	reportedTypeNames  map[types.Object]bool
	coveredTypeTerms   map[ast.Expr]bool
	preferTypeTerms    bool
}

// reportIteratorTypes scans every root before resolving pending constraints and
// returns expressions already covered by either diagnostic path.
func reportIteratorTypes(
	pass *analysis.Pass,
	preferTypeTerms bool,
	roots ...ast.Node,
) map[ast.Expr]bool {
	reporter := iteratorTypeReporter{
		pass:               pass,
		reportedTypeParams: make(map[*types.TypeParam]bool),
		reportedTypeNames:  make(map[types.Object]bool),
		coveredTypeTerms:   make(map[ast.Expr]bool),
		preferTypeTerms:    preferTypeTerms,
	}
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
		case *ast.Field:
			reported = reporter.walkField(node) || reported
			return false
		case ast.Expr:
			if reporter.reportExpression(node) {
				reported = true
			}
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
	if reported && !reporter.preferTypeTerms {
		return true
	}

	constraint := iteratorConstraint{root: field.Type}
	for _, name := range field.Names {
		reporter.collectConstraintTypeParam(&constraint, name, reported)
	}
	if reported || len(constraint.typeParams) == 0 {
		return reported
	}

	// Named interface constraints expose their iterator shape only through
	// go/types, so defer the fallback until the rest of the declaration is scanned.
	reporter.pendingConstraints = append(reporter.pendingConstraints, constraint)
	return false
}

func (reporter *iteratorTypeReporter) collectConstraintTypeParam(
	constraint *iteratorConstraint,
	name *ast.Ident,
	fieldReported bool,
) {
	typeName, ok := reporter.pass.TypesInfo.Defs[name].(*types.TypeName)
	if !ok {
		return
	}
	typeParam, ok := typeName.Type().(*types.TypeParam)
	if !ok {
		return
	}

	iterator := isIteratorType(typeParam)
	if reporter.preferTypeTerms && (fieldReported || iterator) {
		reporter.reportedTypeParams[typeParam] = true
		reporter.reportedTypeNames[typeName] = true
	}
	if !iterator {
		return
	}
	if constraint.position == token.NoPos {
		constraint.position = name.Pos()
	}
	constraint.typeParams = append(constraint.typeParams, typeParam)
}

func (reporter *iteratorTypeReporter) reportExpression(expr ast.Expr) bool {
	if reporter.coveredTypeTerms[expr] {
		coverEquivalentTypeExpressions(reporter.coveredTypeTerms, expr)
		return false
	}
	typ := reporter.pass.TypesInfo.TypeOf(expr)
	if typeParam, ok := types.Unalias(typ).(*types.TypeParam); ok && reporter.preferTypeTerms {
		if ident, ok := expr.(*ast.Ident); ok && reporter.reportedTypeNames[reporter.pass.TypesInfo.Uses[ident]] {
			return false
		}
		if reporter.typeParamWasReported([]*types.TypeParam{typeParam}) {
			return false
		}
	}
	if !reportIteratorType(reporter.pass, expr.Pos(), typ) {
		return false
	}
	if typeParam, ok := types.Unalias(typ).(*types.TypeParam); ok {
		reporter.reportedTypeParams[typeParam] = true
	}
	reporter.coveredTypeTerms[expr] = true
	coverEquivalentTypeExpressions(reporter.coveredTypeTerms, expr)
	return true
}

func (reporter *iteratorTypeReporter) reportPendingConstraints() {
	for _, constraint := range reporter.pendingConstraints {
		if reporter.preferTypeTerms {
			reporter.coveredTypeTerms[constraint.root] = true
			reportIteratorType(reporter.pass, constraint.position, constraint.typeParams[0])
			continue
		}

		reporter.coveredTypeTerms[constraint.root] = true
		if reporter.typeParamWasReported(constraint.typeParams) {
			continue
		}
		reportIteratorType(reporter.pass, constraint.position, constraint.typeParams[0])
	}
}

func (reporter *iteratorTypeReporter) typeParamWasReported(typeParams []*types.TypeParam) bool {
	for _, typeParam := range typeParams {
		for reported := range reporter.reportedTypeParams {
			if typeParam == reported || types.Identical(typeParam, reported) {
				return true
			}
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
			expr, ok := node.(ast.Expr)
			if !ok {
				return true
			}
			if covered[expr] {
				coverEquivalentTypeExpressions(covered, expr)
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
		case *ast.UnaryExpr:
			if expr.Op == token.TILDE {
				next = expr.X
			}
		}
		if next == nil {
			return
		}
		covered[next] = true
		expr = next
	}
}

func iteratorTermType(pass *analysis.Pass, expr ast.Expr) types.Type {
	typeName := referencedTypeName(pass, expr)
	if typeName == nil {
		return nil
	}

	typ := pass.TypesInfo.TypeOf(expr)
	switch typ.(type) {
	case *types.Alias, *types.Named:
	default:
		if !typeName.IsAlias() {
			return nil
		}
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
		return types.Identical(types.Unalias(candidate), types.Typ[types.Bool])
	})
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

func hasOnlyAcceptedSignatures(
	typeParam *types.TypeParam,
	accept func(*types.Signature) bool,
) bool {
	return hasOnlyAcceptedTypes(typeParam, func(candidate types.Type) bool {
		signature, ok := types.Unalias(candidate).Underlying().(*types.Signature)
		return ok && accept(signature)
	})
}

func hasOnlyAcceptedTypes(
	typeParam *types.TypeParam,
	accept func(types.Type) bool,
) bool {
	constraint, ok := typeParam.Constraint().Underlying().(*types.Interface)
	if !ok {
		return false
	}
	constraint.Complete()

	// Exact terms are checked directly. An eligible ~T term needs a named
	// witness carrying the constraint's methods; types that cannot be receiver
	// bases remain direct candidates. Satisfies then filters methods,
	// comparability, and intersections. No survivor is not a universal match.
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
	if typ == nil {
		return
	}

	typ = types.Unalias(typ)
	if seen[typ] {
		return
	}
	seen[typ] = true

	switch typ := typ.(type) {
	case *types.TypeParam:
		collectTypeSetTerms(typ.Constraint(), seen, terms)
	case *types.Named:
		if constraint, ok := typ.Underlying().(*types.Interface); ok {
			collectTypeSetTerms(constraint, seen, terms)
			return
		}
		appendTypeSetTerm(types.NewTerm(false, typ), terms)
	case *types.Interface:
		collectInterfaceTypeSetTerms(typ, seen, terms)
	case *types.Union:
		collectUnionTypeSetTerms(typ, seen, terms)
	default:
		appendTypeSetTerm(types.NewTerm(false, typ), terms)
	}
}

func collectInterfaceTypeSetTerms(
	constraint *types.Interface,
	seen map[types.Type]bool,
	terms *[]*types.Term,
) {
	constraint.Complete()
	// Interface.Empty reports the all-types set, which has no finite leaf
	// candidates to prove universally accepted.
	if constraint.Empty() {
		return
	}
	for index := range constraint.NumEmbeddeds() {
		collectTypeSetTerms(constraint.EmbeddedType(index), seen, terms)
	}
}

func collectUnionTypeSetTerms(
	union *types.Union,
	seen map[types.Type]bool,
	terms *[]*types.Term,
) {
	// The wrapper is Empty only when the union denotes the all-types set.
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
		return underlying.Kind() != types.Invalid &&
			underlying.Kind() != types.UnsafePointer &&
			underlying.Info()&types.IsUntyped == 0
	default:
		return false
	}
}

func newMethodWitness(
	typeParam *types.TypeParam,
	underlying types.Type,
	constraint *types.Interface,
) types.Type {
	pkg := typeParam.Obj().Pkg()
	canAddMethods := true
	var privatePkg *types.Package
	privatePkgSet := false
	for index := range constraint.NumMethods() {
		method := constraint.Method(index)
		if pkg == nil {
			pkg = method.Pkg()
		}
		if method.Exported() {
			continue
		}
		if !privatePkgSet {
			privatePkg = method.Pkg()
			privatePkgSet = true
			continue
		}
		if !samePackage(privatePkg, method.Pkg()) {
			canAddMethods = false
		}
	}
	if privatePkgSet {
		pkg = privatePkg
	}

	typeName := types.NewTypeName(token.NoPos, pkg, "boringlintWitness", nil)
	witness := types.NewNamed(typeName, underlying, nil)
	if !canAddMethods {
		return witness
	}
	for index := range constraint.NumMethods() {
		method := constraint.Method(index)
		signature, ok := method.Type().(*types.Signature)
		if !ok {
			return nil
		}
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

func validatedMethodWitness(witness *types.Named, constraint *types.Interface) types.Type {
	// Named.AddMethod permits synthetic field/method collisions that Go source
	// does not. The computed method set filters those impossible witnesses.
	methodSet := types.NewMethodSet(witness)
	for index := range constraint.NumMethods() {
		method := constraint.Method(index)
		if methodSet.Lookup(method.Pkg(), method.Name()) == nil {
			return nil
		}
	}
	return witness
}

func samePackage(left, right *types.Package) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Path() == right.Path()
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
		"type parameters remain allowed; use a package-level generic function instead. " +
		"A driver built with Go 1.27 or newer is required to inspect Go 1.27 generic-method syntax.",
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
