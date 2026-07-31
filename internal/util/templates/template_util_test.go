//go:build cssstub

package templates

import (
	"embed"
	"strings"
	"testing"
	texttemplate "text/template"
)

//go:embed testdata/*.tmpl
var testTemplates embed.FS

func TestTemplateUtil_Execute(t *testing.T) {
	util := NewTemplateUtil()
	templateStr := "Hello, {{.Name}}!"
	data := struct{ Name string }{"World"}
	result, err := util.Execute(templateStr, data)
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result != "Hello, World!" {
		t.Errorf("Execute() = %v, want %v", result, "Hello, World!")
	}
}

func TestTemplateUtil_AddFunc(t *testing.T) {
	util := NewTemplateUtil()
	util.AddFunc("uppercase", func(s string) string {
		return strings.ToUpper(s)
	})
	templateStr := "Hello, {{uppercase .Name}}!"
	data := struct{ Name string }{"World"}
	result, err := util.Execute(templateStr, data)
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result != "Hello, WORLD!" {
		t.Errorf("Execute() = %v, want %v", result, "Hello, WORLD!")
	}
}

func TestTemplateUtil_AddFuncMap(t *testing.T) {
	util := NewTemplateUtil()
	funcMap := texttemplate.FuncMap{
		"uppercase": strings.ToUpper,
		"lowercase": strings.ToLower,
	}
	util.AddFuncMap(funcMap)
	templateStr := "Hello, {{uppercase .Name}} and {{lowercase .Name}}!"
	data := struct{ Name string }{"World"}
	result, err := util.Execute(templateStr, data)
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result != "Hello, WORLD and world!" {
		t.Errorf("Execute() = %v, want %v", result, "Hello, WORLD and world!")
	}
}

func TestTemplateUtil_ExecuteFile(t *testing.T) {
	util := NewTemplateUtil()
	data := struct{ Name string }{"World"}
	result, err := util.ExecuteFile("testdata/hello.tmpl", data)
	if err != nil {
		t.Errorf("ExecuteFile() error = %v", err)
	}
	if result != "Hello, World!" {
		t.Errorf("ExecuteFile() = %v, want %v", result, "Hello, World!")
	}
}

func TestTemplateUtil_ExecuteFiles(t *testing.T) {
	util := NewTemplateUtil()
	data := struct{ Name string }{"World"}
	result, err := util.ExecuteFiles([]string{"testdata/hello.tmpl", "testdata/goodbye.tmpl"}, data)
	if err != nil {
		t.Errorf("ExecuteFiles() error = %v", err)
	}
	if result != "Hello, World!\nGoodbye, World!" {
		t.Errorf("ExecuteFiles() = %v, want %v", result, "Hello, World!\nGoodbye, World!")
	}
}

func TestTemplateUtil_ExecuteGlob(t *testing.T) {
	util := NewTemplateUtil()
	data := struct{ Name string }{"World"}
	result, err := util.ExecuteGlob("testdata/*.tmpl", data)
	if err != nil {
		t.Errorf("ExecuteGlob() error = %v", err)
	}
	if result != "Hello, World!\nGoodbye, World!" {
		t.Errorf("ExecuteGlob() = %v, want %v", result, "Hello, World!\nGoodbye, World!")
	}
}

func TestTemplateUtil_ExecuteTemplate(t *testing.T) {
	util := NewTemplateUtil()
	tmpl, err := util.Parse(`
		{{define "hello"}}Hello, {{.Name}}!{{end}}
		{{define "goodbye"}}Goodbye, {{.Name}}!{{end}}
	`)
	if err != nil {
		t.Errorf("Parse() error = %v", err)
	}
	data := struct{ Name string }{"World"}
	result, err := util.ExecuteTemplate(tmpl, "hello", data)
	if err != nil {
		t.Errorf("ExecuteTemplate() error = %v", err)
	}
	if result != "Hello, World!" {
		t.Errorf("ExecuteTemplate() = %v, want %v", result, "Hello, World!")
	}
}

func TestTemplateSet(t *testing.T) {
	util := NewTemplateUtil()
	ts := util.NewTemplateSet()

	// Test Add
	err := ts.Add("hello", "Hello, {{.Name}}!")
	if err != nil {
		t.Errorf("Add() error = %v", err)
	}

	// Test Execute
	data := struct{ Name string }{"World"}
	result, err := ts.Execute("hello", data)
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result != "Hello, World!" {
		t.Errorf("Execute() = %v, want %v", result, "Hello, World!")
	}

	// Test AddFile
	err = ts.AddFile("goodbye", "testdata/goodbye.tmpl")
	if err != nil {
		t.Errorf("AddFile() error = %v", err)
	}

	// Test ExecuteTemplate
	result, err = ts.ExecuteTemplate("goodbye", "goodbye", data)
	if err != nil {
		t.Errorf("ExecuteTemplate() error = %v", err)
	}
	if result != "Goodbye, World!" {
		t.Errorf("ExecuteTemplate() = %v, want %v", result, "Goodbye, World!")
	}
}

func TestTemplateUtil_LoadTemplates(t *testing.T) {
	util := NewTemplateUtil()
	ts, err := util.LoadTemplates(testTemplates, "*.tmpl")
	if err != nil {
		t.Errorf("LoadTemplates() error = %v", err)
	}

	data := struct{ Name string }{"World"}
	result, err := ts.Execute("hello", data)
	if err != nil {
		t.Errorf("Execute() error = %v", err)
	}
	if result != "Hello, World!" {
		t.Errorf("Execute() = %v, want %v", result, "Hello, World!")
	}
}
