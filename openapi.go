// Copyright 2020 Torben Schinke
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package http

import (
	"fmt"
	v3 "github.com/golangee/openapi/v3"
	"github.com/golangee/reflectplus"
	"strconv"
	"strings"
)

// MakeDoc tries to generate the OpenAPI documentation from all given controller structs
func MakeDoc(doc *v3.Document, controllers []reflectplus.Struct) error {
	if doc.Components == nil {
		doc.Components = &v3.Components{}
	}

	if doc.Components.Schemas == nil {
		doc.Components.Schemas = map[string]v3.Schema{}
	}

	for _, ctr := range controllers {
		if !reflectplus.Annotations(ctr.Annotations).Has(AnnotationStereotypeController) {
			continue
		}

		err := makeDocController(doc, ctr)
		if err != nil {
			return err
		}
	}

	return nil
}

// makeDocController is partially a copy paste but we want that generation without actual go types, just based on
// our parser reflect data and not really only at runtime.
func makeDocController(doc *v3.Document, meta reflectplus.Struct) error {
	prefixRoutes := httpRoutes(meta.Annotations)

	stereotypeCtr := meta.GetAnnotations().FindFirst(AnnotationStereotypeController)
	oaiGroupTag := ""
	if stereotypeCtr != nil {
		oaiGroupTag = stereotypeCtr.Value()
	}

	for _, m := range meta.Methods {
		method := m
		verbs := httpMethods(method.Annotations)
		routes := httpRoutes(method.Annotations)

		if len(verbs) == 0 {
			continue
		}

		if len(routes) == 0 && len(prefixRoutes) == 0 {
			continue
		}

		if len(routes) == 0 {
			routes = append(routes, "/")
		}

		methodParams, err := scanMethodParams(meta, method)
		if err != nil {
			return reflectplus.PositionalError(method, err)
		}

		for _, prefixRoute := range prefixRoutes {
			for _, route := range routes {
				for _, verb := range verbs {
					path := joinPaths(prefixRoute, route)
					oasPath := pathVarsToOASPath(path)

					item := newPathDoc(doc, verb, path, oaiGroupTag, method, methodParams)

					doc.Paths[oasPath] = item

				}
			}
		}

	}

	return nil
}

func pathVarsToOASPath(path string) string {
	return regexParamNames.ReplaceAllStringFunc(string(path), func(s string) string {
		return "{" + s[1:] + "}"
	})
}

func newPathDoc(doc *v3.Document, verb, path string, tag string, method reflectplus.Method, methodParams []methodParam) v3.PathItem {
	item := v3.PathItem{}
	op := v3.Operation{}
	op.Tags = append(op.Tags, tag)
	op.Summary = reflectplus.DocShortText(method.Doc)
	op.Description = reflectplus.DocText(method.Doc)

	for _, param := range methodParams {
		p := v3.Parameter{}
		p.Name = param.Alias()
		p.Description = paramDoc(param.param)
		ignore := true
		switch param.paramType {
		case ptPath:
			ignore = false
			p.In = v3.PathLocation
			p.Required = true
		case ptQuery:
			ignore = false
			p.In = v3.QueryLocation
		case ptHeader:
			ignore = false
			p.In = v3.HeaderLocation
		}

		if ignore {
			continue
		}

		p.Schema = toSchema(doc, param.param.Type)

		op.Parameters = append(op.Parameters, p)
	}

	op.Responses = map[string]v3.Response{}
	for _, param := range method.Returns {
		if param.Type.ImportPath == "" && param.Type.Identifier == "error" {
			continue //TODO, how to define errors typesafe?
		}

		op.Responses["200"] = v3.Response{
			Description: paramDoc(param),
			Content: map[string]v3.MediaType{
				"application/json": {Schema: toSchema(doc, param.Type)},
			},
		}
	}

	switch strings.ToUpper(verb) {
	case "GET":
		item.Get = &op
	case "DELETE":
		item.Delete = &op
	case "PATCH":
		item.Patch = &op
	case "POST":
		item.Post = &op
	case "PUT":
		item.Put = &op
	default:
		panic("verb not implemented " + verb)
	}
	return item
}

func paramDoc(decl reflectplus.Param) string {
	strct := reflectplus.FindStruct(decl.Type.ImportPath, decl.Type.Identifier)
	if strct == nil && decl.Type.ImportPath == "" && decl.Type.Identifier == "[]" {
		strct = reflectplus.FindStruct(decl.Type.Params[0].ImportPath, decl.Type.Params[0].Identifier)
	}
	doc := decl.Doc + "\n"
	if strct != nil {
		doc = strct.Doc
	}
	return strings.TrimSpace(doc)
}

func toSchema(doc *v3.Document, decl reflectplus.TypeDecl) v3.Schema {
	s := v3.Schema{}
	switch decl.ImportPath {
	case "":
		switch decl.Identifier {
		case "int32":
			fallthrough
		case "int":
			s.Type = v3.Integer
			s.Format = "int32"
		case "int64":
			s.Type = v3.Integer
			s.Format = "int64"
		case "float32":
			s.Type = v3.Number
			s.Format = "float"
		case "float64":
			s.Type = v3.Number
			s.Format = "double"
		case "string":
			s.Type = v3.String
		case "[]":
			tmp := toSchema(doc, decl.Params[0])
			s.Type = v3.Array
			s.Items = &v3.Items{
				Schema: &tmp,
			}
		default:
			panic("cannot emit base type " + decl.Identifier)
		}
	default:
		strct := reflectplus.FindStruct(decl.ImportPath, decl.Identifier)
		if strct == nil {
			panic("return type must be a struct or base type but is " + fmt.Sprintf("%+v", decl))
		}
		if doc.Components == nil {
			doc.Components = &v3.Components{Schemas: map[string]v3.Schema{}}
		}
		xid := decl.ImportPath + "#" + decl.Identifier

		shortId := ""
		has := false
		for k, v := range doc.Components.Schemas {
			if v.XType != nil && *v.XType == xid {
				has = true
				shortId = k
				break
			}
		}

		if has {
			ref := "#/components/schemas/" + shortId
			s.Ref = &ref
			return s
		}

		shortId = uniqueShortId(doc, decl.ImportPath, decl.Identifier)
		newSpec := v3.Schema{}
		newSpec.Type = v3.Object
		newSpec.Description = strct.Doc
		newSpec.XType = &xid
		newSpec.Properties = map[string]v3.Schema{}
		for _, f := range strct.Fields {
			schema := toSchema(doc, f.Type)
			schema.Description = f.Doc
			newSpec.Properties[f.Name] = schema
		}
		ref := "#/components/schemas/" + shortId
		s.Ref = &ref

		doc.Components.Schemas[shortId] = newSpec
	}

	return s
}

func uniqueShortId(doc *v3.Document, importPath, id string) string {
	shortId := id
	i := 2
	for _, has := doc.Components.Schemas[shortId]; has; {
		shortId += strconv.Itoa(i)
		i++
	}
	return shortId
}
