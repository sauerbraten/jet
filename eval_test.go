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
	"bytes"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
	"text/template"
)

// mock data

type User struct {
	Name, Email string
}

func (user *User) Format(str string) string {
	return fmt.Sprintf(str, user.Name, user.Email)
}

func (user *User) GetName() string {
	return user.Name
}

var users = []*User{
	{"Mario Santos", "mario@gmail.com"},
	{"Joel Silva", "joelsilva@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
	{"Mario Santos", "mario@gmail.com"},
	{"Joel Silva", "joelsilva@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
	{"Mario Santos", "mario@gmail.com"},
	{"Joel Silva", "joelsilva@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
	{"Mario Santos", "mario@gmail.com"},
	{"Joel Silva", "joelsilva@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
	{"Mario Santos", "mario@gmail.com"},
	{"Joel Silva", "joelsilva@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
	{"Mario Santos", "mario@gmail.com"},
	{"Joel Silva", "joelsilva@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
	{"Luis Santana", "luis.santana@gmail.com"},
}

// setup

func prepareJet(tb testing.TB, path, content string) *Set {
	set := NewSet(nil, "")

	_, err := set.Cache(path, content)
	if err != nil {
		tb.Fatal(err)
	}

	return set
}

func run(t *testing.T, tmplPath, tmplContent string, vars VarMap, context interface{}, expected string) {
	set := prepareJet(t, tmplPath, tmplContent)
	runWithSet(t, tmplPath, set, vars, context, expected)
}

func runWithSet(t *testing.T, tmplPath string, set *Set, vars VarMap, context interface{}, expected string) {
	tmpl, err := set.GetTemplate(tmplPath)
	if err != nil {
		t.Errorf("error getting template %s: %v", tmplPath, err)
		return
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, vars, context)
	if err != nil {
		t.Errorf("error executing %s: %v", tmplPath, err)
		return
	}

	output := buf.String()
	output = strings.Replace(output, "\r\n", "\n", -1)

	if output != expected {
		t.Errorf("in %s: expected %q, but got %q", tmplPath, expected, output)
	}
}

func mustCache(t *testing.T, set *Set, path, content string) {
	_, err := set.Cache(path, content)
	if err != nil {
		t.Errorf("could not cache template %s (%s) in set: %v", path, content, err)
	}
}

func TestEvalTextNode(t *testing.T) {
	run(t, "/text", `hello {*Buddy*} World`, nil, nil, `hello  World`)
}

func TestEvalActionNode(t *testing.T) {
	var data = make(VarMap)

	data.Set("user", &User{
		"José Santos", "email@example.com",
	})

	run(t, "/action", `hello {{"world"}}`, nil, nil, `hello world`)

	run(t, "/func", `hello {{lower: "WORLD"}}`, data, nil, `hello world`)
	run(t, "/func/pipe", `hello {{lower: "WORLD" |upper}}`, data, nil, `hello WORLD`)
	run(t, "/func/pipe_with_arg", `hello {{lower: "WORLD-" |upper|repeat: 2}}`, data, nil, `hello WORLD-WORLD-`)
	run(t, "/var/field", `Oi {{ user.Name }}`, data, nil, `Oi José Santos`)
	run(t, "/var/method", `Oi {{ user.Format: "%s<%s>" }}`, data, nil, `Oi José Santos<email@example.com>`)

	run(t, "/negative_number", `{{ -5 }}`, nil, nil, "-5")

	run(t, "/add/simple", `{{ 2+1 }}`, nil, nil, fmt.Sprint(2+1))
	run(t, "/add/multiple", `{{ 2+1+4 }}`, nil, nil, fmt.Sprint(2+1+4))
	run(t, "/add/multiple_with_sub", `{{ 2+1+4-3 }}`, nil, nil, fmt.Sprint(2+1+4-3))
	run(t, "/add/int_and_string", `{{ 2+"1" }}`, nil, nil, "3")
	run(t, "/add/string_and_int", `{{ "1"+2 }}`, nil, nil, "12")
	run(t, "/add/negative_number", `{{ 1 + -5 }}`, nil, nil, fmt.Sprint(1+-5))

	run(t, "/mult/simple", `{{ 4*4 }}`, nil, nil, fmt.Sprint(4*4))
	run(t, "/mult/after_add", `{{ 2+4*4 }}`, nil, nil, fmt.Sprint(2+4*4))
	run(t, "/mult/before_add", `{{ 4*2+4 }}`, nil, nil, fmt.Sprint(4*2+4))
	run(t, "/mult/between_add", `{{ 2+4*2+4 }}`, nil, nil, fmt.Sprint(2+4*2+4))
	run(t, "/mult/float", `{{ 1.23*1 }}`, nil, nil, fmt.Sprint(1*1.23))
	run(t, "/mod/simple", `{{ 3%2 }}`, nil, nil, fmt.Sprint(3%2))
	run(t, "/mult/before_mod", `{{ (1*3)%2 }}`, nil, nil, fmt.Sprint((1*3)%2))
	run(t, "/mult/before_div_mod", `{{ (2*5)/ 3 %1 }}`, nil, nil, fmt.Sprint((2*5)/3%1))

	run(t, "/comparison/num/eq", `{{ (2*5)==10 }}`, nil, nil, fmt.Sprint((2*5) == 10))
	run(t, "/comparison/num/neq", `{{ (2*5)==5 }}`, nil, nil, fmt.Sprint((2*5) == 5))
	run(t, "/comparison/bool/eq", `{{ (2*5)==5 || false }}`, nil, nil, fmt.Sprint((2*5) == 5 || false))
	run(t, "/comparison/bool/neq", `{{ (2*5)==5 || true }}`, nil, nil, fmt.Sprint((2*5) == 5 || true))

	run(t, "/comparison/num/gt", `{{ 5*5 > 2*12.5 }}`, nil, nil, fmt.Sprint(5*5 > 2*12.5))
	run(t, "/comparison/num/gte", `{{ 5*5 >= 2*12.5 }}`, nil, nil, fmt.Sprint(5*5 >= 2*12.5))

	run(t, "/comparison/mixed", `{{ 5 * 5 > 2 * 12.5 == 5 * 5 > 2 * 12.5 }}`, nil, nil, fmt.Sprint((5*5 > 2*12.5) == (5*5 > 2*12.5)))
}

func TestEvalIf(t *testing.T) {
	data := VarMap{}.Set("user", &User{
		Name:  "José Santos",
		Email: "email@example.com",
	})

	run(t, "/if", `{{if true}}hello{{end}}`, data, nil, `hello`)
	run(t, "/if/else", `{{if false}}hello{{else}}world{{end}}`, data, nil, `world`)
	run(t, "/if/elseif", `{{if false}}hello{{else if true}}world{{end}}`, data, nil, `world`)
	run(t, "/if/elseif/else", `{{if false}}hello{{else if false}}world{{else}}buddy{{end}}`, data, nil, `buddy`)
	run(t, "/if_string/else", `{{user.Name}} (email: {{user.Email}}): {{if user.Email == "email2@example.com"}}email is email2@example.com{{else}}email is not email2@example.com{{end}}`, data, nil, `José Santos (email: email@example.com): email is not email2@example.com`)
}

func TestEvalBlockYieldIncludeNode(t *testing.T) {

	vars := VarMap{}.Set("user", &User{
		"José Santos", "email@example.com",
	})

	set := prepareJet(t, "/block", `{{block hello() "Buddy" }}Hello {{ . }}{{end}}`)

	runWithSet(t, "/block", set, vars, nil, `Hello Buddy`)

	mustCache(t, set, "/block_yield", `{{block hello() "Buddy" }}Hello {{ . }}{{end}}, {{yield hello() user.Name}}`)
	runWithSet(t, "/block_yield", set, vars, nil, `Hello Buddy, Hello José Santos`)

	mustCache(t, set, "/extend/block_yield/block", `{{extends "/block_yield"}}{{block hello() "Buddy" }}Hey {{ . }}{{end}}`)
	runWithSet(t, "/extend/block_yield/block", set, vars, nil, `Hey Buddy, Hey José Santos`)

	mustCache(t, set, "/import/block/yield", `{{import "/block"}}{{yield hello() "Buddy"}}`)
	runWithSet(t, "/import/block/yield", set, vars, nil, `Hello Buddy`)

	mustCache(t, set, "/yield", `{{yield hello() "Buddy"}}`)
	mustCache(t, set, "/import/block/include/yield", `{{import "/block"}}{{include "/yield"}}`)
	runWithSet(t, "/import/block/include/yield", set, vars, nil, `Hello Buddy`)

	mustCache(t, set, "/block/yield_content", `{{ block foo(bar=2) }}bar: {{ bar }} content: {{ yield content }}{{ end }}, {{ block header() }}{{ yield foo(bar=4) content }}some content{{ end }}{{ end }}`)
	runWithSet(t, "/block/yield_content", set, vars, nil, `bar: 2 content: , bar: 4 content: some content`)
}

func TestEvalRange(t *testing.T) {
	users := []User{
		{"Mario Santos", "mario@gmail.com"},
		{"Joel Silva", "joelsilva@gmail.com"},
		{"Luis Santana", "luis.santana@gmail.com"},
	}

	vars := VarMap{}.Set("users", users)

	run(t, "/range/var_as_context",
		`{{range users}}{{.Name}}: {{.Email}}; {{end}}`,
		vars, nil,
		`Mario Santos: mario@gmail.com; Joel Silva: joelsilva@gmail.com; Luis Santana: luis.santana@gmail.com; `)
	run(t, "/range/var_as_var",
		`{{range u := users}}{{u.Name}}: {{u.Email}}; {{end}}`,
		vars, nil,
		`Mario Santos: mario@gmail.com; Joel Silva: joelsilva@gmail.com; Luis Santana: luis.santana@gmail.com; `)
	run(t, "/range/context_as_context",
		`{{range .}}{{.Name}}: {{.Email}}; {{end}}`,
		nil, users,
		`Mario Santos: mario@gmail.com; Joel Silva: joelsilva@gmail.com; Luis Santana: luis.santana@gmail.com; `)
	run(t, "/range/context_as_var",
		`{{range u := .}}{{u.Name}}: {{u.Email}}; {{end}}`,
		nil, users,
		`Mario Santos: mario@gmail.com; Joel Silva: joelsilva@gmail.com; Luis Santana: luis.santana@gmail.com; `)
}

func TestEvalDefaults(t *testing.T) {
	run(t, "/defaults/len/string/literal", `{{len("111")}}`, nil, nil, "3")
	run(t, "/defaults/len/slice/context", `{{len(.)}}`, nil, []int{1, 2, 3}, "3")

	run(t, "/defaults/safe_html", `<h1>{{"<h1>Hello Buddy!</h1>" |safeHtml}}</h1>`, nil, nil, `<h1>&lt;h1&gt;Hello Buddy!&lt;/h1&gt;</h1>`)
	run(t, "/defaults/safe_html/2", `<h1>{{safeHtml: "<h1>Hello Buddy!</h1>"}}</h1>`, nil, nil, `<h1>&lt;h1&gt;Hello Buddy!&lt;/h1&gt;</h1>`)
	run(t, "/defaults/html_escape", `<h1>{{html: "<h1>Hello Buddy!</h1>"}}</h1>`, nil, nil, `<h1>&lt;h1&gt;Hello Buddy!&lt;/h1&gt;</h1>`)
	run(t, "/defaults/url_escape", `<h1>{{url: "<h1>Hello Buddy!</h1>"}}</h1>`, nil, nil, `<h1>%3Ch1%3EHello+Buddy%21%3C%2Fh1%3E</h1>`)

	run(t, "/defaults/write_json", `{{. |writeJson}}`, nil, &User{"Mario Santos", "mario@gmail.com"}, "{\"Name\":\"Mario Santos\",\"Email\":\"mario@gmail.com\"}\n")

	run(t, "/defaults/replace", `{{replace("My Name Is", " ", "_", -1)}}`, nil, nil, "My_Name_Is")
	run(t, "/defaults/replace/multiline",
		`{{replace("My Name Is II",
			" ",
			"_",
			-1,
		)}}`, nil, nil,
		"My_Name_Is_II",
	)

	vars := VarMap{}.Set("title", "title")
	run(t, "/defaults/isset/var/fail", `{{isset(value)}}`, vars, nil, "false")
	run(t, "/defaults/isset/var/ok", `{{isset(title)}}`, vars, nil, "true")
	run(t, "/defaults/isset/var/field/fail", `{{isset(title.Get)}}`, vars, nil, "false")

	user := &User{
		"José Santos", "email@example.com",
	}
	run(t, "/defaults/isset/context/fail", `{{isset(.NotSet)}}`, nil, user, "false")
	run(t, "/defaults/isset/context/ok", `{{isset(.Name)}}`, nil, user, "true")
	run(t, "/defaults/isset/context/field/fail", `{{isset(.Name.NotSet)}}`, nil, user, "false")

	context := map[string]interface{}{
		"foo": map[string]interface{}{
			"asd": map[string]string{
				"bar": "baz",
			},
		},
	}
	run(t, "/defaults/isset/context/nested", `{{isset(.foo)}}`, nil, context, "true")
	run(t, "/defaults/isset/context/nested/2", `{{isset(.foo.asd)}}`, nil, context, "true")
	run(t, "/defaults/isset/context/nested/3", `{{isset(.foo.asd.bar)}}`, nil, context, "true")
	run(t, "/defaults/isset/context/nested/fail", `{{isset(.asd)}}`, nil, context, "false")
	run(t, "/defaults/isset/context/nested/fail/2", `{{isset(.foo.bar)}}`, nil, context, "false")
	run(t, "/defaults/isset/context/nested/fail/3", `{{isset(.foo.asd.foo)}}`, nil, context, "false")
	run(t, "/defaults/isset/context/nested/fail/4", `{{isset(.foo.asd.bar.xyz)}}`, nil, context, "false")
}

func TestEvalTernaryExpr(t *testing.T) {
	vars := VarMap{}.
		Set("yes", true).
		Set("no", false)

	run(t, "/ternary/fail", `{{no ? "yes" : "no"}}`, vars, nil, "no")
	run(t, "/ternary/ok", `{{yes ? "yes" : "no"}}`, vars, nil, "yes")
	// todo: make this work:
	// run(t, "/ternary/unset_var", `{{not_set ? "yes" : "no"}}`, vars, nil, "no")
}

func TestEvalIndexExpr(t *testing.T) {
	abc := "abc"
	// run(t, "/index/string/context", `{{.[1]}}`, nil, abc, `b`)
	// run(t, "/index/string/var", `{{abc[1]}}`, VarMap{}.Set("abc", abc), nil, `b`)

	abcdef := []string{"abc", "def"}
	run(t, "/index/slice/context", `{{.[1]}}`, nil, abcdef, `def`)
	run(t, "/index/slice/var", `{{abcdef[1]}}`, VarMap{}.Set("abcdef", abcdef), nil, `def`)

	abcdefghijkl := [][]string{{"abc", "def"}, {"ghi", "jkl"}}
	run(t, "/index/slice/slice/context", `{{.[1][0]}}`, nil, abcdefghijkl, `ghi`)
	run(t, "/index/slice/slice/var", `{{abcdefghijkl[1][0]}}`, VarMap{}.Set("abcdefghijkl", abcdefghijkl), nil, `ghi`)

	m := map[string]string{"name": "value"}

	run(t, "/index/map/brackets/context/ok", `{{.["name"]}}`, nil, m, "value")
	run(t, "/index/map/brackets/context/fail", `{{.["non_existant_key"]}}`, nil, m, "")
	run(t, "/index/map/brackets/context/two_values/ok", `{{ v, found := .["name"] }}'{{isset(v) ? v : ""}}', {{found}}`, nil, m, "'value', true")
	run(t, "/index/map/brackets/context/two_values/fail", `{{ v, found := .["not_in_map"] }}'{{isset(v) ? v : ""}}', {{found}}`, nil, m, "'', false")

	run(t, "/index/map/brackets/var/ok", `{{m["name"]}}`, VarMap{}.Set("m", m), nil, "value")
	run(t, "/index/map/brackets/var/fail", `{{m["non_existant_key"]}}`, VarMap{}.Set("m", m), nil, "")
	run(t, "/index/map/brackets/var/two_values/ok", `{{ v, found := m["name"] }}'{{isset(v) ? v : ""}}', {{found}}`, VarMap{}.Set("m", m), nil, "'value', true")
	run(t, "/index/map/brackets/var/two_values/fail", `{{ v, found := m["not_in_map"] }}'{{isset(v) ? v : ""}}', {{found}}`, VarMap{}.Set("m", m), nil, "'', false")

	user := User{"José Santos", "email@example.com"}

	run(t, "/index/struct_context/brackets/field_name", `{{.["Email"]}}`, nil, user, "email@example.com")
	run(t, "/index/struct_context/dots/field_name", `{{.Email}}`, nil, user, "email@example.com")

	nested := map[string]map[string]map[string]map[string]map[string]interface{}{
		"one": {
			"two": {
				"three": {
					"four": {
						"abc":          abc,
						"abcdef":       abcdef,
						"abcdefghijkl": abcdefghijkl,
					},
				},
			},
		},
	}

	run(t, "/index/nested/dots/map/string", `{{.one.two.three.four.abc}}`, nil, nested, "abc")
	run(t, "/index/nested/dots/map/slice", `{{.one.two.three.four.abcdef[0]}}`, nil, nested, "abc")
	run(t, "/index/nested/dots/map/slice/slice", `{{.one.two.three.four.abcdefghijkl[0][1]}}`, nil, nested, "def")
	run(t, "/index/nested/mixed/map/string", `{{.one.two.three.four["abc"]}}`, nil, nested, "abc")
	run(t, "/index/nested/mixed/map/string", `{{.one.two.three["four"].abc}}`, nil, nested, "abc")
	run(t, "/index/nested/mixed/map/string", `{{.one.two["three"].four["abc"]}}`, nil, nested, "abc")
	run(t, "/index/nested/mixed/map/string", `{{.one["two"].three.four.abc}}`, nil, nested, "abc")
	run(t, "/index/nested/mixed/map/string", `{{.["one"].two["three"].four["abc"]}}`, nil, nested, "abc")
}

func TestEvalSliceExpr(t *testing.T) {
	s := []string{"111", "222", "333", "444"}

	run(t, "/slice/1_to_end", `{{range .[1:]}}{{.}}{{end}}`, nil, s, `222333444`)
	run(t, "/slice/start_to_2", `{{range .[:2]}}{{.}}{{end}}`, nil, s, `111222`)
	run(t, "/slice/start_to_end", `{{range .[:]}}{{.}}{{end}}`, nil, s, `111222333444`)
	run(t, "/slice/0_to_2", `{{range .[0:2]}}{{.}}{{end}}`, nil, s, `111222`)
	run(t, "/slice/1_to_2", `{{range .[1:2]}}{{.}}{{end}}`, nil, s, `222`)
	run(t, "/slice/1_to_3", `{{range .[1:3]}}{{.}}{{end}}`, nil, s, `222333`)
}

type stringer struct{}

func (s *stringer) String() string { return "implements fmt.Stringer" }

func TestEvalPointerExpr(t *testing.T) {
	vars := VarMap{}.
		// Set("n", (interface{})(nil)).
		Set("s", (*string)(nil)).
		Set("i", (*int)(nil))

	// todo:
	// run(t, "/ptr/nil/interface", `{{ n }}`, vars, nil, "<nil>")
	run(t, "/ptr/nil/string", `{{ s }}`, vars, nil, "<nil>")
	run(t, "/ptr/nil/int", `{{ i }}`, vars, nil, "<nil>")

	s := "foo"
	s_ := &s
	s__ := &s_
	s___ := &s__

	vars = VarMap{}.
		Set("s", s).
		Set("s_", s_).
		Set("s__", s__).
		Set("s___", s___)

	run(t, "/ptr/string", `{{ s }}`, vars, nil, "foo")
	run(t, "/ptr/string/1", `{{ s_ }}`, vars, nil, "foo")
	run(t, "/ptr/string/2", `{{ s__ }}`, vars, nil, "foo")
	// run(t, "/ptr/string/3", `{{ s___ }}`, vars, nil, "foo")

	i := 10
	i_ := &i
	i__ := &i_
	i___ := &i__
	vars = VarMap{}.
		Set("i", i).
		Set("i_", i_).
		Set("i__", i__).
		Set("i___", i___)

	run(t, "/ptr/int", `{{ i }}`, vars, nil, "10")
	run(t, "/ptr/int/1", `{{ i_ }}`, vars, nil, "10")
	run(t, "/ptr/int/2", `{{ i__ }}`, vars, nil, "10")
	// run(t, "/ptr/int/3", `{{ i___ }}`, vars, nil, "10")

	st := stringer{}
	run(t, "/ptr/stringer", `{{ st }}`, VarMap{}.Set("st", st), nil, "{}")

	st_ := &st
	run(t, "/ptr/stringer/1", `{{ st_ }}`, VarMap{}.Set("st_", st_), nil, "implements fmt.Stringer")

	st__ := &st_
	run(t, "/ptr/stringer/2", `{{ st__ }}`, VarMap{}.Set("st__", st__), nil, "implements fmt.Stringer")

	st___ := &st__
	run(t, "/ptr/stringer/3", `{{ st___ }}`, VarMap{}.Set("st___", st___), nil, "implements fmt.Stringer")

	u := User{
		Name:  "Pablo",
		Email: "pablo@escobar.mx",
	}
	u_ := &u
	u__ := &u_
	u___ := &u__
	vars = VarMap{}.
		Set("u", u).
		Set("u_", u_).
		Set("u__", u__).
		Set("u___", u___)

	run(t, "/ptr/struct", `{{ u }}`, vars, nil, "{Pablo pablo@escobar.mx}")
	run(t, "/ptr/struct/1", `{{ u_ }}`, vars, nil, "{Pablo pablo@escobar.mx}")
	run(t, "/ptr/struct/2", `{{ u__ }}`, vars, nil, "{Pablo pablo@escobar.mx}")
	run(t, "/ptr/struct/3", `{{ u___ }}`, vars, nil, "&{Pablo pablo@escobar.mx}")
}

func TestEvalStructPointerFields(t *testing.T) {
	type ptrStruct struct {
		String *string
		Int    *int
		Struct *ptrStruct
	}

	someString := "test"
	someInt := 10
	innerString := "nested"

	vars := VarMap{}.Set("s", ptrStruct{
		String: &someString,
		Int:    &someInt,
		Struct: &ptrStruct{
			String: &innerString,
		},
	})

	run(t, "/ptr/struct/string", `{{ s.String }}`, vars, nil, "test")
	run(t, "/ptr/struct/int", `{{ s.Int }}`, vars, nil, "10")
	run(t, "/ptr/struct/struct/string", `{{ s.Struct.String }}`, vars, nil, "nested")
	run(t, "/ptr/struct/struct/int/nil", `{{ s.Struct.Int }}`, vars, nil, "<nil>")

	vars = VarMap{}.Set("s", ptrStruct{
		// all fields are nil
	})

	run(t, "/ptr/struct/int/nil", `{{ s.Int }}`, vars, nil, "<nil>")
	run(t, "/ptr/struct/string/nil", `{{ s.String }}`, vars, nil, "<nil>")
	run(t, "/ptr/struct/struct/nil", `{{ s.Struct }}`, vars, nil, "<nil>")
}

func TestEvalAutoescape(t *testing.T) {
	set := NewHTMLSet("")
	mustCache(t, set, "/autoescape/1", `<h1>{{"<h1>Hello Buddy!</h1>" }}</h1>`)
	runWithSet(t, "/autoescape/1", set, nil, nil, "<h1>&lt;h1&gt;Hello Buddy!&lt;/h1&gt;</h1>")
	mustCache(t, set, "/autoescape/2", `<h1>{{"<h1>Hello Buddy!</h1>" |unsafe }}</h1>`)
	runWithSet(t, "/autoescape/2", set, nil, nil, "<h1><h1>Hello Buddy!</h1></h1>")
}

func TestFileResolve(t *testing.T) {
	set := NewHTMLSet("./testData/resolve")
	runWithSet(t, "/simple", set, nil, nil, "simple")
	runWithSet(t, "/simple.jet", set, nil, nil, "simple.jet")
	runWithSet(t, "/extension", set, nil, nil, "extension.jet.html")
	runWithSet(t, "/extension.jet.html", set, nil, nil, "extension.jet.html")
	runWithSet(t, "./sub/subextend", set, nil, nil, "simple - simple.jet - extension.jet.html")
	runWithSet(t, "./sub/extend", set, nil, nil, "simple - simple.jet - extension.jet.html")
	//for key, _ := range set.templates {
	//	t.Log(key)
	//}
}

func TestIncludeIfNotExists(t *testing.T) {
	set := NewHTMLSet("./testData/includeIfNotExists")
	runWithSet(t, "/existent", set, nil, nil, "Hi, i exist!!")
	runWithSet(t, "/notExistent", set, nil, nil, "")
	runWithSet(t, "/ifIncludeIfExits", set, nil, nil, "Hi, i exist!!\n    Was included!!\n\n\n    Was not included!!\n\n")
	runWithSet(t, "/wcontext", set, nil, "World", "Hi, Buddy!\nHi, World!")

	// Check if includeIfExists helper bubbles up runtime errors of included templates
	tt, err := set.GetTemplate("/includeBroken")
	if err != nil {
		t.Error(err)
	}
	buff := bytes.NewBuffer(nil)
	err = tt.Execute(buff, nil, nil)
	if err == nil {
		t.Error("expected includeIfExists helper to fail with a runtime error but got nil")
	}
}

// benchmarks

func dummy(a string) string {
	return a
}

func prepareStd(tb testing.TB, path, content string) *template.Template {
	std, err := template.New(path).Parse(content)
	if err != nil {
		tb.Fatal(err)
	}

	return std
}

func BenchmarkSimpleAction(b *testing.B) {
	set := prepareJet(b, "/action/dummy", `hello {{dummy("WORLD")}}`)
	set.AddGlobal("dummy", dummy)

	b.ResetTimer()

	t, _ := set.GetTemplate("/action/dummy")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ioutil.Discard, nil, nil)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkSimpleActionStd(b *testing.B) {
	std := template.New("/action/dummy")
	std.Funcs(template.FuncMap{"dummy": dummy})
	_, err := std.Parse(`hello {{dummy "WORLD"}}`)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	t := std.Lookup("/action/dummy")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ioutil.Discard, nil)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkSimpleActionNoAlloc(b *testing.B) {
	set := prepareJet(b, "/no_allocs", `hello {{ "José" }} {{1}} {{ "José" }}`)
	b.ResetTimer()
	t, _ := set.GetTemplate("/no_allocs")
	for i := 0; i < b.N; i++ {
		t.Execute(ioutil.Discard, nil, nil)
	}
}

func BenchmarkSimpleActionNoAllocStd(b *testing.B) {
	std := prepareStd(b, "/no_allocs", `hello {{ "José" }} {{1}} {{ "José" }}`)
	b.ResetTimer()
	t := std.Lookup("/no_allocs")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ioutil.Discard, nil)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkRangeSimple(b *testing.B) {
	set := prepareJet(b, "/range/context/simple", `{{range .}}{{.Name}} - {{.Email}}{{end}}`)
	b.ResetTimer()
	t, _ := set.GetTemplate("/range/context/simple")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ioutil.Discard, nil, &users)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkRangeSimpleStd(b *testing.B) {
	std := prepareStd(b, "/range/context/simple", `{{range .}}{{.Name}} - {{.Email}}{{end}}`)
	t := std.Lookup("/range/context/simple")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ioutil.Discard, &users)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkRangeIndexed(b *testing.B) {
	set := prepareJet(b, "/range/context/indexed", `{{range i, u := .}}{{i}}: {{u.Name}} - {{u.Email}}{{end}}`)
	b.ResetTimer()
	t, _ := set.GetTemplate("/range/context/indexed")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ioutil.Discard, nil, &users)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkRangeIndexedStd(b *testing.B) {
	std := prepareStd(b, "/range/context/indexed", `{{range $i, $v := .}}{{$i}}: {{$v.Name}} - {{$v.Email}}{{end}}`)
	t := std.Lookup("/range/context/indexed")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ioutil.Discard, &users)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkNewBlockYield(b *testing.B) {
	set := prepareJet(b, "/block_yield", `
{{ block col(md=12, offset=0) }}
	<div class="col-md-{{md}} col-md-offset-{{offset}}">{{ yield content }}</div>
{{ end }}
{{ block row(md=12) }}
	<div class="row {{md}}">{{ yield content }}</div>
	{{ content }}
	<div class="col-md-1"></div>
	<div class="col-md-1"></div>
	<div class="col-md-1"></div>
{{ end }}
{{ block header() }}
	<div class="header">
	{{ yield row() content }}
		{{ yield col(md=6) content }}
			{{ yield content }}
		{{ end }}
	{{ end }}
	</div>
	<h1>Hey {{ content }}!</h1>
{{ end }}
{{ yield header() "You" }}`)
	b.ResetTimer()
	t, _ := set.GetTemplate("/block_yield")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ioutil.Discard, nil, &users)
		if err != nil {
			panic(err)
		}
	}
}

func BenchmarkFuncDyn(b *testing.B) {
	set := prepareJet(b, "/func/dyn", `hello {{dummy("WORLD")}}`)
	vars := VarMap{}.Set("dummy", dummy)
	b.ResetTimer()

	t, _ := set.GetTemplate("/func/dyn")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ioutil.Discard, vars, nil)
		if err != nil {
			b.Error(err.Error())
		}
	}
}

func BenchmarkFuncFast(b *testing.B) {
	set := prepareJet(b, "/func/fast", `hello {{dummy("WORLD")}}`)
	vars := VarMap{}.SetFunc("dummy", func(a Arguments) reflect.Value {
		return reflect.ValueOf(dummy(a.Get(0).String()))
	})
	b.ResetTimer()

	t, _ := set.GetTemplate("/func/fast")
	for i := 0; i < b.N; i++ {
		err := t.Execute(ioutil.Discard, vars, nil)
		if err != nil {
			b.Error(err.Error())
		}
	}
}
