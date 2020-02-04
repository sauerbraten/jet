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
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"net/url"
	"reflect"
	"strings"
	"text/template"
)

var defaultVariables map[string]reflect.Value

func init() {
	defaultVariables = map[string]reflect.Value{
		"lower":     reflect.ValueOf(strings.ToLower),
		"upper":     reflect.ValueOf(strings.ToUpper),
		"hasPrefix": reflect.ValueOf(strings.HasPrefix),
		"hasSuffix": reflect.ValueOf(strings.HasSuffix),
		"repeat":    reflect.ValueOf(strings.Repeat),
		"replace":   reflect.ValueOf(strings.Replace),
		"split":     reflect.ValueOf(strings.Split),
		"trimSpace": reflect.ValueOf(strings.TrimSpace),
		"map":       reflect.ValueOf(newMap),
		"html":      reflect.ValueOf(html.EscapeString),
		"url":       reflect.ValueOf(url.QueryEscape),
		"safeHtml":  reflect.ValueOf(EscapeFunc(template.HTMLEscape)),
		"safeJs":    reflect.ValueOf(EscapeFunc(template.JSEscape)),
		"raw":       reflect.ValueOf(EscapeFunc(unsafePrinter)),
		"unsafe":    reflect.ValueOf(EscapeFunc(unsafePrinter)),
		"writeJson": reflect.ValueOf(jsonRenderer),
		"json":      reflect.ValueOf(json.Marshal),
		"isset": reflect.ValueOf(Func(func(a Arguments) reflect.Value {
			a.RequireNumOfArguments("isset", 1, -1)
			for i := 0; i < len(a.argExpr); i++ {
				if !a.runtime.isSet(a.argExpr[i]) {
					return reflect.ValueOf(false)
				}
			}
			return reflect.ValueOf(true)
		})),
		"len": reflect.ValueOf(Func(func(a Arguments) reflect.Value {
			a.RequireNumOfArguments("len", 1, 1)

			expression := a.Get(0)
			if expression.Kind() == reflect.Ptr || expression.Kind() == reflect.Interface {
				expression = expression.Elem()
			}

			switch expression.Kind() {
			case reflect.Array, reflect.Chan, reflect.Slice, reflect.Map, reflect.String:
				return reflect.ValueOf(expression.Len())
			case reflect.Struct:
				return reflect.ValueOf(expression.NumField())
			}

			a.Panicf("invalid value type %s in len builtin", expression.Type())
			return reflect.Value{}
		})),
		"includeIfExists": reflect.ValueOf(Func(func(a Arguments) reflect.Value {
			a.RequireNumOfArguments("includeIfExists", 1, 2)

			execContext := a.runtime.context
			if a.NumOfArguments() > 1 {
				execContext = a.Get(1)
			}

			_, err := a.runtime.execute(a.Get(0).String(), a.runtime.writer, execContext)
			if err != nil {
				var notFound templateNotFoundErr
				if errors.As(err, &notFound) {
					return hiddenFalse
				}
				// template exists but returns an error -> panic instead of failing silently
				panic(err)
			}

			return hiddenTrue
		})),
		"exec": reflect.ValueOf(Func(func(a Arguments) (result reflect.Value) {
			a.RequireNumOfArguments("exec", 1, 2)

			execContext := a.runtime.context
			if a.NumOfArguments() > 1 {
				execContext = a.Get(1)
			}

			result, err := a.runtime.execute(a.Get(0).String(), ioutil.Discard, execContext)
			if err != nil {
				panic(err)
			}

			return result
		})),
	}
}

type hiddenBool bool

func (m hiddenBool) Render(r *Runtime) { /* */ }

var hiddenTrue = reflect.ValueOf(hiddenBool(true))
var hiddenFalse = reflect.ValueOf(hiddenBool(false))

func jsonRenderer(v interface{}) RendererFunc {
	return func(r *Runtime) {
		err := json.NewEncoder(r.writer).Encode(v)
		if err != nil {
			panic(err)
		}
	}
}

func newMap(values ...interface{}) map[string]interface{} {
	if len(values)%2 > 0 {
		panic(fmt.Errorf("map(): expected even number of arguments, but got %d", len(values)))
	}

	m := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		m[fmt.Sprint(values[i])] = values[i+1]
	}
	return m
}
