package jet

import (
	"io"
	"reflect"
	"sync"
)

// Runtime this type holds the state of the execution of an template
type Runtime struct {
	set    *Set
	writer io.Writer
	*scope
	content func(*Runtime, Expression)
	context reflect.Value
}

type scope struct {
	parent    *scope
	variables VarMap
	blocks    map[string]*BlockNode
}

func (st *scope) getBlock(name string) (block *BlockNode, has bool) {
	block, has = st.blocks[name]
	for !has && st.parent != nil {
		st = st.parent
		block, has = st.blocks[name]
	}
	return
}

// Context returns the current context value
func (r *Runtime) Context() reflect.Value {
	return r.context
}

func (st *Runtime) enterScope() {
	st.scope = &scope{parent: st.scope, variables: make(VarMap), blocks: st.blocks}
}

func (st *Runtime) exitScope() {
	st.scope = st.scope.parent
}

// Set sets variable ${name} in the current template scope
func (rt *Runtime) Set(name string, val interface{}) {
	rt.setValue(name, reflect.ValueOf(val))
}

func (rt *Runtime) setValue(name string, val reflect.Value) {
	sc := rt.scope

	// try to find a variable with the given name
	_, ok := sc.variables[name]
	for !ok && sc.parent != nil {
		sc = sc.parent
		_, ok = sc.variables[name]
	}

	if ok {
		// set variable where it was found
		sc.variables[name] = val
		return
	}

	// set variable in original, current scope
	rt.scope.variables[name] = val
}

// Resolve resolves a value from the execution context
func (rt *Runtime) Resolve(name string) reflect.Value {
	if name == "." {
		return rt.context
	}

	sc := rt.scope
	// try to find a variable with the given name
	val, ok := sc.variables[name]
	for !ok && sc.parent != nil {
		sc = sc.parent
		val, ok = sc.variables[name]
	}

	// if not found check globals
	if !ok {
		rt.set.gmx.RLock()
		val, ok = rt.set.globals[name]
		rt.set.gmx.RUnlock()
		// not found check defaultVariables
		if !ok {
			val = defaultVariables[name]
		}
	}
	return val
}

var runtimes = sync.Pool{
	New: func() interface{} {
		return &Runtime{scope: &scope{}}
	},
}

func prepareRuntime(s *Set, vars VarMap) (*Runtime, func()) {
	rt := runtimes.Get().(*Runtime)
	rt.variables = vars
	rt.set = s

	return rt, func() {
		rt.scope = &scope{}
		runtimes.Put(rt)
	}
}
