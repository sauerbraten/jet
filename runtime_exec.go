package jet

import (
	"io"
	"reflect"

	"github.com/CloudyKit/fastprinter"
)

func (rt *Runtime) execute(templatePath string, w io.Writer, context reflect.Value) (result reflect.Value, err error) {
	t, err := rt.set.GetTemplate(templatePath)
	if err != nil {
		return reflect.Value{}, err
	}

	// setup execution writer, scope, context
	backedUpWriter := rt.writer
	rt.writer = w
	rt.enterScope()
	rt.blocks = t.processedBlocks
	root := t.Root
	if t.extends != nil {
		root = t.extends.Root
	}
	backedUpContext, execContext := rt.context, rt.context
	if context.IsValid() {
		execContext = context
	}
	rt.context = execContext

	// run template statements
	result = rt.executeList(root)

	// restore context, scope, writer
	rt.context = backedUpContext
	rt.exitScope()
	rt.writer = backedUpWriter

	return
}

func (rt *Runtime) executeList(list *ListNode) reflect.Value {
	inNewSCOPE := false
	defer func() {
		if inNewSCOPE {
			rt.exitScope()
		}
	}()

	returnValue := reflect.Value{}

	for i := 0; i < len(list.Nodes); i++ {
		node := list.Nodes[i]
		switch node.Type() {

		case NodeText:
			node := node.(*TextNode)
			_, err := rt.writer.Write(node.Text)
			if err != nil {
				node.error(err)
			}
		case NodeAction:
			node := node.(*ActionNode)
			if node.Set != nil {
				if node.Set.Let {
					if !inNewSCOPE {
						rt.enterScope() //creates new scope in the back state
						inNewSCOPE = true
					}
					rt.executeLetList(node.Set)
				} else {
					rt.executeSetList(node.Set)
				}
			}
			if node.Pipe != nil {
				v, printed := rt.evalPipelineExpression(node.Pipe)
				if !printed && v.IsValid() {
					if v.Type().Implements(rendererType) {
						v.Interface().(Renderer).Render(rt)
					} else {
						_, err := fastprinter.PrintValue(&EscapeWriter{w: rt.writer, escape: rt.set.escape}, v)
						if err != nil {
							node.error(err)
						}
					}
				}
			}
		case NodeIf:
			node := node.(*IfNode)
			var isLet bool
			if node.Set != nil {
				if node.Set.Let {
					isLet = true
					rt.enterScope()
					rt.executeLetList(node.Set)
				} else {
					rt.executeSetList(node.Set)
				}
			}

			if isTrue(rt.evalPrimaryExpressionGroup(node.Expression)) {
				returnValue = rt.executeList(node.List)
			} else if node.ElseList != nil {
				returnValue = rt.executeList(node.ElseList)
			}
			if isLet {
				rt.exitScope()
			}
		case NodeRange:
			node := node.(*RangeNode)
			var expression reflect.Value

			isSet := node.Set != nil
			isLet := false
			isKeyVal := false

			context := rt.context

			if isSet {
				isKeyVal = len(node.Set.Left) > 1
				expression = rt.evalPrimaryExpressionGroup(node.Set.Right[0])
				if node.Set.Let {
					isLet = true
					rt.enterScope()
				}
			} else {
				expression = rt.evalPrimaryExpressionGroup(node.Expression)
			}

			ranger := getRanger(expression)
			indexValue, rangeValue, end := ranger.Range()
			if !end {
				for !end && !returnValue.IsValid() {
					if isSet {
						if isLet {
							if isKeyVal {
								rt.variables[node.Set.Left[0].String()] = indexValue
								rt.variables[node.Set.Left[1].String()] = rangeValue
							} else {
								rt.variables[node.Set.Left[0].String()] = rangeValue
							}
						} else {
							if isKeyVal {
								rt.executeSet(node.Set.Left[0], indexValue)
								rt.executeSet(node.Set.Left[1], rangeValue)
							} else {
								rt.executeSet(node.Set.Left[0], rangeValue)
							}
						}
					} else {
						rt.context = rangeValue
					}
					returnValue = rt.executeList(node.List)
					indexValue, rangeValue, end = ranger.Range()
				}
			} else if node.ElseList != nil {
				returnValue = rt.executeList(node.ElseList)
			}
			rt.context = context
			if isLet {
				rt.exitScope()
			}
		case NodeYield:
			node := node.(*YieldNode)
			if node.IsContent {
				if rt.content != nil {
					rt.content(rt, node.Expression)
				}
			} else {
				block, has := rt.getBlock(node.Name)
				if has == false || block == nil {
					node.errorf("unresolved block %q!!", node.Name)
				}
				rt.executeYieldBlock(block, block.Parameters, node.Parameters, node.Expression, node.Content)
			}
		case NodeBlock:
			node := node.(*BlockNode)
			block, has := rt.getBlock(node.Name)
			if has == false {
				block = node
			}
			rt.executeYieldBlock(block, block.Parameters, block.Parameters, block.Expression, block.Content)
		case NodeInclude:
			node := node.(*IncludeNode)

			var path string
			_path := rt.evalPrimaryExpressionGroup(node.Name)
			if _path.Type().Implements(stringerType) {
				path = _path.String()
			} else if _path.Kind() == reflect.String {
				path = _path.String()
			} else {
				node.errorf("unexpected expression type %q in template yielding", getTypeString(_path))
			}
			path = rt.set.resolvePath(path, node.TemplatePath)

			context := reflect.Value{}
			if node.Expression != nil {
				context = rt.evalPrimaryExpressionGroup(node.Expression)
			}

			var err error
			returnValue, err = rt.execute(path, rt.writer, context)
			if err != nil {
				node.error(err)
			}
		case NodeReturn:
			node := node.(*ReturnNode)
			returnValue = rt.evalPrimaryExpressionGroup(node.Value)
		}
	}

	return returnValue
}

func (rt *Runtime) executeSetList(set *SetNode) {
	if set.IndexExprGetLookup {
		value := rt.evalPrimaryExpressionGroup(set.Right[0])
		rt.executeSet(set.Left[0], value)
		if value.IsValid() {
			rt.executeSet(set.Left[1], reflect.ValueOf(true))
		} else {
			rt.executeSet(set.Left[1], reflect.ValueOf(false))
		}
	} else {
		for i := 0; i < len(set.Left); i++ {
			rt.executeSet(set.Left[i], rt.evalPrimaryExpressionGroup(set.Right[i]))
		}
	}
}

func (rt *Runtime) executeLetList(set *SetNode) {
	if set.IndexExprGetLookup {
		value := rt.evalPrimaryExpressionGroup(set.Right[0])

		rt.variables[set.Left[0].(*IdentifierNode).Ident] = value

		if value.IsValid() {
			rt.variables[set.Left[1].(*IdentifierNode).Ident] = reflect.ValueOf(true)
		} else {
			rt.variables[set.Left[1].(*IdentifierNode).Ident] = reflect.ValueOf(false)
		}

	} else {
		for i := 0; i < len(set.Left); i++ {
			rt.variables[set.Left[i].(*IdentifierNode).Ident] = rt.evalPrimaryExpressionGroup(set.Right[i])
		}
	}
}

func (rt *Runtime) executeSet(left Expression, right reflect.Value) {
	typ := left.Type()
	if typ == NodeIdentifier {
		rt.setValue(left.(*IdentifierNode).Ident, right)
		return
	}
	var value reflect.Value
	var fields []string
	if typ == NodeChain {
		chain := left.(*ChainNode)
		value = rt.evalPrimaryExpressionGroup(chain.Node)
		fields = chain.Field
	} else {
		fields = left.(*FieldNode).Ident
		value = rt.context
	}
	lef := len(fields) - 1
	for i := 0; i < lef; i++ {
		var err error
		value, err = resolveIndex(value, reflect.ValueOf(fields[i]))
		if err != nil {
			left.errorf("%v", err)
		}
	}

RESTART:
	switch value.Kind() {
	case reflect.Ptr:
		value = value.Elem()
		goto RESTART
	case reflect.Struct:
		value = value.FieldByName(fields[lef])
		if !value.IsValid() {
			left.errorf("identifier %q is not available in the current scope", fields[lef])
		}
		value.Set(right)
	case reflect.Map:
		value.SetMapIndex(reflect.ValueOf(&fields[lef]).Elem(), right)
	}
}

func (rt *Runtime) executeYieldBlock(block *BlockNode, blockParam, yieldParam *BlockParameterList, expression Expression, content *ListNode) {

	needNewScope := len(blockParam.List) > 0 || len(yieldParam.List) > 0
	if needNewScope {
		rt.enterScope()
		for i := 0; i < len(yieldParam.List); i++ {
			p := &yieldParam.List[i]
			rt.variables[p.Identifier] = rt.evalPrimaryExpressionGroup(p.Expression)
		}
		for i := 0; i < len(blockParam.List); i++ {
			p := &blockParam.List[i]
			if _, found := rt.variables[p.Identifier]; !found {
				if p.Expression == nil {
					rt.variables[p.Identifier] = reflect.ValueOf(false)
				} else {
					rt.variables[p.Identifier] = rt.evalPrimaryExpressionGroup(p.Expression)
				}
			}
		}
	}

	mycontent := rt.content
	if content != nil {
		myscope := rt.scope
		rt.content = func(st *Runtime, expression Expression) {
			outscope := rt.scope
			outcontent := rt.content

			rt.scope = myscope
			rt.content = mycontent

			if expression != nil {
				context := rt.context
				rt.context = rt.evalPrimaryExpressionGroup(expression)
				rt.executeList(content)
				rt.context = context
			} else {
				rt.executeList(content)
			}

			rt.scope = outscope
			rt.content = outcontent
		}
	}

	if expression != nil {
		context := rt.context
		rt.context = rt.evalPrimaryExpressionGroup(expression)
		rt.executeList(block.List)
		rt.context = context
	} else {
		rt.executeList(block.List)
	}

	rt.content = mycontent
	if needNewScope {
		rt.exitScope()
	}
}
