package main

import (
	"encoding/json"
	"text/template"
)

var extraTemplateFuncs template.FuncMap = template.FuncMap{
	"json": func(v interface{}) (string, error) {
		bytes, err := json.Marshal(v)
		return string(bytes), err
	},
}
