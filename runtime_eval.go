// Copyright 2016 José Santos <henrique_1609@me.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jet

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/CloudyKit/fastprinter"
)

var (
	funcType       = reflect.TypeOf(Func(nil))
	stringerType   = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()
	rangerType     = reflect.TypeOf((*Ranger)(nil)).Elem()
	rendererType   = reflect.TypeOf((*Renderer)(nil)).Elem()
	escapeFuncType = reflect.TypeOf(EscapeFunc(nil))
)

// Renderer any resulting value from an expression in an action that implements this
// interface will not be printed, instead, we will invoke his Render() method which will be responsible
// to render his self
type Renderer interface {
	Render(*Runtime)
}

// RendererFunc func implementing interface Renderer
type RendererFunc func(*Runtime)

func (renderer RendererFunc) Render(r *Runtime) {
	renderer(r)
}

func (rt *Runtime) evalPrimaryExpressionGroup(node Expression) reflect.Value {
	switch node.Type() {
	case NodeAdditiveExpr:
		return rt.evalAdditiveExpression(node.(*AdditiveExprNode))
	case NodeMultiplicativeExpr:
		return rt.evalMultiplicativeExpression(node.(*MultiplicativeExprNode))
	case NodeComparativeExpr:
		return rt.evalComparativeExpression(node.(*ComparativeExprNode))
	case NodeNumericComparativeExpr:
		return rt.evalNumericComparativeExpression(node.(*NumericComparativeExprNode))
	case NodeLogicalExpr:
		return rt.evalLogicalExpression(node.(*LogicalExprNode))
	case NodeNotExpr:
		return reflect.ValueOf(!isTrue(rt.evalPrimaryExpressionGroup(node.(*NotExprNode).Expr)))
	case NodeTernaryExpr:
		node := node.(*TernaryExprNode)
		if isTrue(rt.evalPrimaryExpressionGroup(node.Boolean)) {
			return rt.evalPrimaryExpressionGroup(node.Left)
		}
		return rt.evalPrimaryExpressionGroup(node.Right)
	case NodeCallExpr:
		node := node.(*CallExprNode)
		baseExpr := rt.evalBaseExpressionGroup(node.BaseExpr)
		if baseExpr.Kind() != reflect.Func {
			node.errorf("node %q is not func kind %q", node.BaseExpr, baseExpr.Type())
		}
		return rt.evalCallExpression(baseExpr, node.Args)
	case NodeIndexExpr:
		node := node.(*IndexExprNode)
		base := rt.evalPrimaryExpressionGroup(node.Base)
		index := rt.evalPrimaryExpressionGroup(node.Index)

		resolved, err := resolveIndex(base, index)
		if err != nil {
			node.error(err)
		}
		return indirectEface(resolved)
	case NodeSliceExpr:
		node := node.(*SliceExprNode)
		baseExpression := rt.evalPrimaryExpressionGroup(node.Base)

		var index, length int
		if node.Index != nil {
			indexExpression := rt.evalPrimaryExpressionGroup(node.Index)
			if canNumber(indexExpression.Kind()) {
				index = int(castInt64(indexExpression))
			} else {
				node.Index.errorf("non numeric value in index expression kind %s", indexExpression.Kind().String())
			}
		}

		if node.EndIndex != nil {
			indexExpression := rt.evalPrimaryExpressionGroup(node.EndIndex)
			if canNumber(indexExpression.Kind()) {
				length = int(castInt64(indexExpression))
			} else {
				node.EndIndex.errorf("non numeric value in index expression kind %s", indexExpression.Kind().String())
			}
		} else {
			length = baseExpression.Len()
		}

		return baseExpression.Slice(index, length)
	}
	return rt.evalBaseExpressionGroup(node)
}

// notNil returns false when v.IsValid() == false
// or when v's kind can be nil and v.IsNil() == true
func notNil(v reflect.Value) bool {
	if !v.IsValid() {
		return false
	}
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return !v.IsNil()
	default:
		return true
	}
}

func (rt *Runtime) isSet(node Node) bool {
	nodeType := node.Type()

	switch nodeType {
	case NodeIndexExpr:
		node := node.(*IndexExprNode)
		if !rt.isSet(node.Base) || !rt.isSet(node.Index) {
			return false
		}

		base := rt.evalPrimaryExpressionGroup(node.Base)
		index := rt.evalPrimaryExpressionGroup(node.Index)

		resolved, err := resolveIndex(base, index)
		return err == nil && notNil(resolved)
	case NodeIdentifier:
		value := rt.Resolve(node.String())
		return notNil(value)
	case NodeField:
		node := node.(*FieldNode)
		resolved := rt.context
		for i := 0; i < len(node.Ident); i++ {
			var err error
			resolved, err = resolveIndex(resolved, reflect.ValueOf(node.Ident[i]))
			if err != nil || !notNil(resolved) {
				return false
			}
		}
	case NodeChain:
		node := node.(*ChainNode)
		resolved, err := rt.evalChainNodeExpression(node)
		return err == nil && notNil(resolved)
	default:
		//todo: maybe work some edge cases
		if !(nodeType > beginExpressions && nodeType < endExpressions) {
			node.errorf("unexpected %q node in isset clause", node)
		}
	}
	return true
}

func (rt *Runtime) evalNumericComparativeExpression(node *NumericComparativeExprNode) reflect.Value {
	left, right := rt.evalPrimaryExpressionGroup(node.Left), rt.evalPrimaryExpressionGroup(node.Right)
	isTrue := false
	kind := left.Kind()

	// if the left value is not a float and the right is, we need to promote the left value to a float before the calculation
	// this is necessary for expressions like 4*1.23
	needFloatPromotion := !isFloat(kind) && isFloat(right.Kind())

	switch node.Operator.typ {
	case itemGreat:
		if isInt(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Int()) > right.Float()
			} else {
				isTrue = left.Int() > toInt(right)
			}
		} else if isFloat(kind) {
			isTrue = left.Float() > toFloat(right)
		} else if isUint(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Uint()) > right.Float()
			} else {
				isTrue = left.Uint() > toUint(right)
			}
		} else {
			node.Left.errorf("a non numeric value in numeric comparative expression")
		}
	case itemGreatEquals:
		if isInt(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Int()) >= right.Float()
			} else {
				isTrue = left.Int() >= toInt(right)
			}
		} else if isFloat(kind) {
			isTrue = left.Float() >= toFloat(right)
		} else if isUint(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Uint()) >= right.Float()
			} else {
				isTrue = left.Uint() >= toUint(right)
			}
		} else {
			node.Left.errorf("a non numeric value in numeric comparative expression")
		}
	case itemLess:
		if isInt(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Int()) < right.Float()
			} else {
				isTrue = left.Int() < toInt(right)
			}
		} else if isFloat(kind) {
			isTrue = left.Float() < toFloat(right)
		} else if isUint(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Uint()) < right.Float()
			} else {
				isTrue = left.Uint() < toUint(right)
			}
		} else {
			node.Left.errorf("a non numeric value in numeric comparative expression")
		}
	case itemLessEquals:
		if isInt(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Int()) <= right.Float()
			} else {
				isTrue = left.Int() <= toInt(right)
			}
		} else if isFloat(kind) {
			isTrue = left.Float() <= toFloat(right)
		} else if isUint(kind) {
			if needFloatPromotion {
				isTrue = float64(left.Uint()) <= right.Float()
			} else {
				isTrue = left.Uint() <= toUint(right)
			}
		} else {
			node.Left.errorf("a non numeric value in numeric comparative expression")
		}
	}
	return reflect.ValueOf(isTrue)
}

func (rt *Runtime) evalLogicalExpression(node *LogicalExprNode) reflect.Value {
	truthy := isTrue(rt.evalPrimaryExpressionGroup(node.Left))
	if node.Operator.typ == itemAnd {
		truthy = truthy && isTrue(rt.evalPrimaryExpressionGroup(node.Right))
	} else {
		truthy = truthy || isTrue(rt.evalPrimaryExpressionGroup(node.Right))
	}
	return reflect.ValueOf(truthy)
}

func (rt *Runtime) evalComparativeExpression(node *ComparativeExprNode) reflect.Value {
	left, right := rt.evalPrimaryExpressionGroup(node.Left), rt.evalPrimaryExpressionGroup(node.Right)
	equal := checkEquality(left, right)
	if node.Operator.typ == itemNotEquals {
		return reflect.ValueOf(!equal)
	}
	return reflect.ValueOf(equal)
}

func toInt(v reflect.Value) int64 {
	kind := v.Kind()
	if isInt(kind) {
		return v.Int()
	} else if isFloat(kind) {
		return int64(v.Float())
	} else if isUint(kind) {
		return int64(v.Uint())
	} else if kind == reflect.String {
		n, e := strconv.ParseInt(v.String(), 10, 0)
		if e != nil {
			panic(e)
		}
		return n
	} else if kind == reflect.Bool {
		if v.Bool() {
			return 0
		}
		return 1
	}
	panic(fmt.Errorf("type: %q can't be converted to int64", v.Type()))
}

func toUint(v reflect.Value) uint64 {
	kind := v.Kind()
	if isUint(kind) {
		return v.Uint()
	} else if isInt(kind) {
		return uint64(v.Int())
	} else if isFloat(kind) {
		return uint64(v.Float())
	} else if kind == reflect.String {
		n, e := strconv.ParseUint(v.String(), 10, 0)
		if e != nil {
			panic(e)
		}
		return n
	} else if kind == reflect.Bool {
		if v.Bool() {
			return 0
		}
		return 1
	}
	panic(fmt.Errorf("type: %q can't be converted to uint64", v.Type()))
}

func toFloat(v reflect.Value) float64 {
	kind := v.Kind()
	if isFloat(kind) {
		return v.Float()
	} else if isInt(kind) {
		return float64(v.Int())
	} else if isUint(kind) {
		return float64(v.Uint())
	} else if kind == reflect.String {
		n, e := strconv.ParseFloat(v.String(), 0)
		if e != nil {
			panic(e)
		}
		return n
	} else if kind == reflect.Bool {
		if v.Bool() {
			return 0
		}
		return 1
	}
	panic(fmt.Errorf("type: %q can't be converted to float64", v.Type()))
}

func (rt *Runtime) evalMultiplicativeExpression(node *MultiplicativeExprNode) reflect.Value {
	left, right := rt.evalPrimaryExpressionGroup(node.Left), rt.evalPrimaryExpressionGroup(node.Right)
	kind := left.Kind()
	// if the left value is not a float and the right is, we need to promote the left value to a float before the calculation
	// this is necessary for expressions like 4*1.23
	needFloatPromotion := !isFloat(kind) && isFloat(right.Kind())
	switch node.Operator.typ {
	case itemMul:
		if isInt(kind) {
			if needFloatPromotion {
				// do the promotion and calculates
				left = reflect.ValueOf(float64(left.Int()) * right.Float())
			} else {
				// do not need float promotion
				left = reflect.ValueOf(left.Int() * toInt(right))
			}
		} else if isFloat(kind) {
			left = reflect.ValueOf(left.Float() * toFloat(right))
		} else if isUint(kind) {
			if needFloatPromotion {
				left = reflect.ValueOf(float64(left.Uint()) * right.Float())
			} else {
				left = reflect.ValueOf(left.Uint() * toUint(right))
			}
		} else {
			node.Left.errorf("a non numeric value in multiplicative expression")
		}
	case itemDiv:
		if isInt(kind) {
			if needFloatPromotion {
				left = reflect.ValueOf(float64(left.Int()) / right.Float())
			} else {
				left = reflect.ValueOf(left.Int() / toInt(right))
			}
		} else if isFloat(kind) {
			left = reflect.ValueOf(left.Float() / toFloat(right))
		} else if isUint(kind) {
			if needFloatPromotion {
				left = reflect.ValueOf(float64(left.Uint()) / right.Float())
			} else {
				left = reflect.ValueOf(left.Uint() / toUint(right))
			}
		} else {
			node.Left.errorf("a non numeric value in multiplicative expression")
		}
	case itemMod:
		if isInt(kind) {
			left = reflect.ValueOf(left.Int() % toInt(right))
		} else if isFloat(kind) {
			left = reflect.ValueOf(int64(left.Float()) % toInt(right))
		} else if isUint(kind) {
			left = reflect.ValueOf(left.Uint() % toUint(right))
		} else {
			node.Left.errorf("a non numeric value in multiplicative expression")
		}
	}
	return left
}

func (rt *Runtime) evalAdditiveExpression(node *AdditiveExprNode) reflect.Value {

	isAdditive := node.Operator.typ == itemAdd
	if node.Left == nil {
		right := rt.evalPrimaryExpressionGroup(node.Right)
		kind := right.Kind()
		// todo: optimize
		// todo:
		if isInt(kind) {
			if isAdditive {
				return reflect.ValueOf(+right.Int())
			} else {
				return reflect.ValueOf(-right.Int())
			}
		} else if isUint(kind) {
			if isAdditive {
				return right
			} else {
				return reflect.ValueOf(-int64(right.Uint()))
			}
		} else if isFloat(kind) {
			if isAdditive {
				return reflect.ValueOf(+right.Float())
			} else {
				return reflect.ValueOf(-right.Float())
			}
		}
		node.Left.errorf("additive expression: right side %s (%s) is not a numeric value (left is nil)", node.Right, getTypeString(right))
	}

	left, right := rt.evalPrimaryExpressionGroup(node.Left), rt.evalPrimaryExpressionGroup(node.Right)
	kind := left.Kind()
	// if the left value is not a float and the right is, we need to promote the left value to a float before the calculation
	// this is necessary for expressions like 4+1.23
	needFloatPromotion := !isFloat(kind) && kind != reflect.String && isFloat(right.Kind())
	if needFloatPromotion {
		if isInt(kind) {
			if isAdditive {
				left = reflect.ValueOf(float64(left.Int()) + right.Float())
			} else {
				left = reflect.ValueOf(float64(left.Int()) - right.Float())
			}
		} else if isUint(kind) {
			if isAdditive {
				left = reflect.ValueOf(float64(left.Uint()) + right.Float())
			} else {
				left = reflect.ValueOf(float64(left.Uint()) - right.Float())
			}
		} else {
			node.Left.errorf("additive expression: left side (%s (%s) needs float promoation but neither int nor uint)", node.Left, getTypeString(left))
		}
	} else {
		if isInt(kind) {
			if isAdditive {
				left = reflect.ValueOf(left.Int() + toInt(right))
			} else {
				left = reflect.ValueOf(left.Int() - toInt(right))
			}
		} else if isFloat(kind) {
			if isAdditive {
				left = reflect.ValueOf(left.Float() + toFloat(right))
			} else {
				left = reflect.ValueOf(left.Float() - toFloat(right))
			}
		} else if isUint(kind) {
			if isAdditive {
				left = reflect.ValueOf(left.Uint() + toUint(right))
			} else {
				left = reflect.ValueOf(left.Uint() - toUint(right))
			}
		} else if kind == reflect.String {
			if isAdditive {
				left = reflect.ValueOf(left.String() + fmt.Sprint(right))
			} else {
				node.Right.errorf("minus signal is not allowed with strings")
			}
		} else {
			node.Left.errorf("additive expression: left side %s (%s) is not a numeric value", node.Left, getTypeString(left))
		}
	}

	return left
}

func getTypeString(value reflect.Value) string {
	if value.IsValid() {
		return value.Type().String()
	}
	return "<invalid>"
}

func (rt *Runtime) evalBaseExpressionGroup(node Node) reflect.Value {
	switch node.Type() {
	case NodeNil:
		return reflect.ValueOf(nil)
	case NodeBool:
		if node.(*BoolNode).True {
			return reflect.ValueOf(true)
		}
		return reflect.ValueOf(false)
	case NodeString:
		return reflect.ValueOf(&node.(*StringNode).Text).Elem()
	case NodeIdentifier:
		resolved := rt.Resolve(node.(*IdentifierNode).Ident)
		if !resolved.IsValid() {
			node.errorf("identifier '%s' is not available in the current scope (%+v)", node, rt.variables)
		}
		return indirectEface(resolved)
	case NodeField:
		node := node.(*FieldNode)
		resolved := rt.context
		for i := 0; i < len(node.Ident); i++ {
			field, err := resolveIndex(resolved, reflect.ValueOf(node.Ident[i]))
			if err != nil {
				node.errorf("%v", err)
			}
			if !field.IsValid() {
				node.errorf("there is no field or method '%s' in %s (.%s)", node.Ident[i], getTypeString(resolved), strings.Join(node.Ident, "."))
			}
			resolved = field
		}
		return indirectEface(resolved)
	case NodeChain:
		resolved, err := rt.evalChainNodeExpression(node.(*ChainNode))
		if err != nil {
			node.error(err)
		}
		return indirectEface(resolved)
	case NodeNumber:
		node := node.(*NumberNode)
		if node.IsFloat {
			return reflect.ValueOf(&node.Float64).Elem()
		}

		if node.IsInt {
			return reflect.ValueOf(&node.Int64).Elem()
		}

		if node.IsUint {
			return reflect.ValueOf(&node.Uint64).Elem()
		}
	}
	node.errorf("unexpected node type %s in unary expression evaluating", node)
	return reflect.Value{}
}

func (rt *Runtime) evalCallExpression(baseExpr reflect.Value, args []Expression, values ...reflect.Value) reflect.Value {
	if funcType.AssignableTo(baseExpr.Type()) {
		return baseExpr.Interface().(Func)(Arguments{runtime: rt, argExpr: args, argVal: values})
	}

	i := len(args) + len(values)
	var returns []reflect.Value
	if i <= 10 {
		returns = reflect_Call10(i, rt, baseExpr, args, values...)
	} else {
		returns = reflect_Call(make([]reflect.Value, i, i), rt, baseExpr, args, values...)
	}

	if len(returns) == 0 {
		return reflect.Value{}
	}

	return returns[0]
}

func (rt *Runtime) evalCommandExpression(node *CommandNode) (reflect.Value, bool) {
	term := rt.evalPrimaryExpressionGroup(node.BaseExpr)
	if node.Call {
		if term.Kind() == reflect.Func {
			if term.Type() == escapeFuncType {
				rt.evalEscapeFunc(term.Interface().(EscapeFunc), node)
				return reflect.Value{}, true
			}
			return rt.evalCallExpression(term, node.Args), false
		} else {
			node.Args[0].errorf("command %q type %s is not func", node.Args[0], term.Type())
		}
	}
	return term, false
}

func (rt *Runtime) evalChainNodeExpression(node *ChainNode) (reflect.Value, error) {
	resolved := rt.evalPrimaryExpressionGroup(node.Node)

	for i := 0; i < len(node.Field); i++ {
		resolved = indirectEface(resolved)
		field, err := resolveIndex(resolved, reflect.ValueOf(node.Field[i]))
		if err != nil {
			return reflect.Value{}, err
		}
		if !field.IsValid() {
			if resolved.Kind() == reflect.Map && i == len(node.Field)-1 {
				// return reflect.Zero(resolved.Type().Elem()), nil
				return reflect.Value{}, nil
			}
			return reflect.Value{}, fmt.Errorf("there is no field or method '%s' in %s (%s)", node.Field[i], getTypeString(resolved), node)
		}
		resolved = field
	}
	return resolved, nil
}

func (rt *Runtime) evalEscapeFunc(escape EscapeFunc, node *CommandNode, values ...reflect.Value) {
	w := &EscapeWriter{
		w:      rt.writer,
		escape: escape,
	}
	for _, v := range values {
		_, err := fastprinter.PrintValue(w, v)
		if err != nil {
			node.error(err)
		}
	}
	for _, arg := range node.Args {
		_, err := fastprinter.PrintValue(w, rt.evalPrimaryExpressionGroup(arg))
		if err != nil {
			node.error(err)
		}
	}
}

func (rt *Runtime) evalCommandPipeExpression(node *CommandNode, value reflect.Value) (reflect.Value, bool) {
	term := rt.evalPrimaryExpressionGroup(node.BaseExpr)
	if term.Kind() == reflect.Func {
		if term.Type() == escapeFuncType {
			rt.evalEscapeFunc(term.Interface().(EscapeFunc), node, value)
			return reflect.Value{}, true
		}
		return rt.evalCallExpression(term, node.Args, value), false
	} else {
		node.BaseExpr.errorf("pipe command %q type %s is not func", node.BaseExpr, term.Type())
	}
	return term, false
}

func (rt *Runtime) evalPipelineExpression(node *PipeNode) (value reflect.Value, safeWriter bool) {
	value, safeWriter = rt.evalCommandExpression(node.Cmds[0])
	for i := 1; i < len(node.Cmds); i++ {
		if safeWriter {
			node.Cmds[i].errorf("unexpected command %s, writer command should be the last command", node.Cmds[i])
		}
		value, safeWriter = rt.evalCommandPipeExpression(node.Cmds[i], value)
	}
	return
}

func reflect_Call(arguments []reflect.Value, rt *Runtime, fn reflect.Value, args []Expression, values ...reflect.Value) []reflect.Value {
	typ := fn.Type()
	numIn := typ.NumIn()

	isVariadic := typ.IsVariadic()
	if isVariadic {
		numIn--
	}
	i, j := 0, 0

	for ; i < numIn && i < len(values); i++ {
		in := typ.In(i)
		term := values[i]
		if !term.Type().AssignableTo(in) {
			term = term.Convert(in)
		}
		arguments[i] = term
	}

	if isVariadic {
		in := typ.In(numIn).Elem()
		for ; i < len(values); i++ {
			term := values[i]
			if !term.Type().AssignableTo(in) {
				term = term.Convert(in)
			}
			arguments[i] = term
		}
	}

	for ; i < numIn && j < len(args); i, j = i+1, j+1 {
		in := typ.In(i)
		term := rt.evalPrimaryExpressionGroup(args[j])
		if !term.Type().AssignableTo(in) {
			term = term.Convert(in)
		}
		arguments[i] = term
	}

	if isVariadic {
		in := typ.In(numIn).Elem()
		for ; j < len(args); i, j = i+1, j+1 {
			term := rt.evalPrimaryExpressionGroup(args[j])
			if !term.Type().AssignableTo(in) {
				term = term.Convert(in)
			}
			arguments[i] = term
		}
	}
	return fn.Call(arguments[0:i])
}

func reflect_Call10(i int, rt *Runtime, fn reflect.Value, args []Expression, values ...reflect.Value) []reflect.Value {
	var arguments [10]reflect.Value
	return reflect_Call(arguments[0:i], rt, fn, args, values...)
}

func isUint(kind reflect.Kind) bool {
	return kind >= reflect.Uint && kind <= reflect.Uint64
}
func isInt(kind reflect.Kind) bool {
	return kind >= reflect.Int && kind <= reflect.Int64
}
func isFloat(kind reflect.Kind) bool {
	return kind == reflect.Float32 || kind == reflect.Float64
}

// checkEquality of two reflect values in the semantic of the jet runtime
func checkEquality(v1, v2 reflect.Value) bool {
	v1 = indirectInterface(v1)
	v2 = indirectInterface(v2)

	if !v1.IsValid() || !v2.IsValid() {
		return v1.IsValid() == v2.IsValid()
	}

	v1Type := v1.Type()
	v2Type := v2.Type()

	// fast path
	if v1Type != v2Type && !v2Type.AssignableTo(v1Type) && !v2Type.ConvertibleTo(v1Type) {
		return false
	}

	kind := v1.Kind()
	if isInt(kind) {
		return v1.Int() == toInt(v2)
	}
	if isFloat(kind) {
		return v1.Float() == toFloat(v2)
	}
	if isUint(kind) {
		return v1.Uint() == toUint(v2)
	}

	switch kind {
	case reflect.Bool:
		return v1.Bool() == isTrue(v2)
	case reflect.String:
		return v1.String() == v2.String()
	case reflect.Array:
		vlen := v1.Len()
		if vlen == v2.Len() {
			return false
		}
		for i := 0; i < vlen; i++ {
			if !checkEquality(v1.Index(i), v2.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Slice:
		if v1.IsNil() != v2.IsNil() {
			return false
		}

		vlen := v1.Len()
		if vlen != v2.Len() {
			return false
		}

		if v1.CanAddr() && v2.CanAddr() && v1.Pointer() == v2.Pointer() {
			return true
		}

		for i := 0; i < vlen; i++ {
			if !checkEquality(v1.Index(i), v2.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Interface:
		if v1.IsNil() || v2.IsNil() {
			return v1.IsNil() == v2.IsNil()
		}
		return checkEquality(v1.Elem(), v2.Elem())
	case reflect.Ptr:
		return v1.Pointer() == v2.Pointer()
	case reflect.Struct:
		numField := v1.NumField()
		for i, n := 0, numField; i < n; i++ {
			if !checkEquality(v1.Field(i), v2.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if v1.IsNil() != v2.IsNil() {
			return false
		}
		if v1.Len() != v2.Len() {
			return false
		}
		if v1.Pointer() == v2.Pointer() {
			return true
		}
		for _, k := range v1.MapKeys() {
			val1 := v1.MapIndex(k)
			val2 := v2.MapIndex(k)
			if !val1.IsValid() || !val2.IsValid() || !checkEquality(v1.MapIndex(k), v2.MapIndex(k)) {
				return false
			}
		}
		return true
	case reflect.Func:
		return v1.IsNil() && v2.IsNil()
	default:
		// Normal equality suffices
		return v1.Interface() == v2.Interface()
	}
}

func isTrue(v reflect.Value) bool {
	return v.IsValid() && !v.IsZero()
}

func canNumber(kind reflect.Kind) bool {
	return isInt(kind) || isUint(kind) || isFloat(kind)
}

func castInt64(v reflect.Value) int64 {
	kind := v.Kind()
	switch {
	case isInt(kind):
		return v.Int()
	case isUint(kind):
		return int64(v.Uint())
	case isFloat(kind):
		return int64(v.Float())
	}
	return 0
}

var cachedStructsMutex = sync.RWMutex{}
var cachedStructsFieldIndex = map[reflect.Type]map[string][]int{}

// from text/template's exec.go:
//
// indirect returns the item at the end of indirection, and a bool to indicate
// if it's nil. If the returned bool is true, the returned value's kind will be
// either a pointer or interface.
func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() {
			return v, true
		}
	}
	return v, false
}

// indirectInterface returns the concrete value in an interface value, or else v itself.
// That is, if v represents the interface value x, the result is the same as reflect.ValueOf(x):
// the fact that x was an interface value is forgotten.
func indirectInterface(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Interface {
		return v.Elem()
	}
	return v
}

// indirectEface is the same as indirectInterface, but only indirects through v if its type
// is the empty interface and its value is not nil.
func indirectEface(v reflect.Value) reflect.Value {
	if v.Kind() == reflect.Interface && v.Type().NumMethod() == 0 && !v.IsNil() {
		return v.Elem()
	}
	return v
}

// mostly copied from text/template's evalField() (exec.go):
func resolveIndex(v, index reflect.Value) (reflect.Value, error) {
	if !v.IsValid() {
		return reflect.Value{}, fmt.Errorf("there is no field or method '%s' in %s (%s)", index, v, getTypeString(v))
	}

	v, isNil := indirect(v)
	if v.Kind() == reflect.Interface && isNil {
		// Calling a method on a nil interface can't work. The
		// MethodByName method call below would panic.
		return reflect.Value{}, fmt.Errorf("nil pointer evaluating %s.%s", v.Type(), index)
	}

	// Unless it's an interface, need to get to a value of type *T to guarantee
	// we see all methods of T and *T.
	if index.Kind() == reflect.String {
		ptr := v
		if ptr.Kind() != reflect.Interface && ptr.Kind() != reflect.Ptr && ptr.CanAddr() {
			ptr = ptr.Addr()
		}
		if method := ptr.MethodByName(index.String()); method.IsValid() {
			return method, nil
		}
	}

	// It's not a method; must be a field of a struct or an element of a map.
	switch v.Kind() {
	case reflect.Array, reflect.Slice, reflect.String:
		x, err := indexArg(index, v.Len())
		if err != nil {
			return reflect.Value{}, err
		}
		return v.Index(x), nil
	case reflect.Struct:
		if index.Kind() != reflect.String {
			return reflect.Value{}, fmt.Errorf("can't use %s (%s, not string) as field name in struct type %s", index, index.Type(), v.Type())
		}
		tField, ok := v.Type().FieldByName(index.String())
		if ok {
			field := v.FieldByIndex(tField.Index)
			if tField.PkgPath != "" { // field is unexported
				return reflect.Value{}, fmt.Errorf("%s is an unexported field of struct type %s", index.String(), v.Type())
			}
			return field, nil
		}
		return reflect.Value{}, fmt.Errorf("can't use %s as field name in struct type %s", index, v.Type())
	case reflect.Map:
		// If it's a map, attempt to use the field name as a key.
		if !index.Type().AssignableTo(v.Type().Key()) {
			return reflect.Value{}, fmt.Errorf("can't use %s (%s) as key for map of type %s", index, index.Type(), v.Type())
		}
		return v.MapIndex(index), nil
	case reflect.Ptr:
		etyp := v.Type().Elem()
		if etyp.Kind() == reflect.Struct && index.Kind() == reflect.String {
			if _, ok := etyp.FieldByName(index.String()); !ok {
				// If there's no such field, say "can't evaluate"
				// instead of "nil pointer evaluating".
				break
			}
		}
		if isNil {
			return reflect.Value{}, fmt.Errorf("nil pointer evaluating %s.%s", v.Type(), index)
		}
	}
	return reflect.Value{}, fmt.Errorf("can't evaluate index %s (%s) in type %s", index, index.Type(), v.Type())
	panic("not reached")
}

// from Go's text/template's funcs.go:
//
// indexArg checks if a reflect.Value can be used as an index, and converts it to int if possible.
func indexArg(index reflect.Value, cap int) (int, error) {
	var x int64
	switch index.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		x = index.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		x = int64(index.Uint())
	case reflect.Float32, reflect.Float64:
		x = int64(index.Float())
	case reflect.Invalid:
		return 0, fmt.Errorf("cannot index slice/array/string with nil")
	default:
		return 0, fmt.Errorf("cannot index slice/array/string with type %s", index.Type())
	}
	if int(x) < 0 || int(x) >= cap {
		return 0, fmt.Errorf("index out of range: %d", x)
	}
	return int(x), nil
}
