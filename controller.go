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
	"database/sql"
	"encoding/json"
	"fmt"
	v3 "github.com/golangee/openapi/v3"
	"github.com/golangee/reflectplus"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// A Scanner is SQL scanner compatible, but only needs tp scan from a string
type Scanner interface {
	Scan(src interface{}) error
}

type Controller struct {
	doc *v3.Document
}

// MustNewController asserts that ctr is useful controller and otherwise bails out.
func MustNewController(srv *Server, ctr interface{}) *Controller {
	c, err := NewController(srv, ctr)
	if err != nil {
		panic(err)
	}
	return c
}

// NewController tries to create a http/rest presentation service/controller/layer from the given instance.
func NewController(srv *Server, ctr interface{}) (*Controller, error) {
	doc := v3.NewDocument()
	res := &Controller{doc: &doc}

	rtype := reflect.TypeOf(ctr)
	if rtype.Kind() == reflect.Ptr {
		rtype = rtype.Elem()
	}
	s := reflectplus.FindByType(rtype)
	if s == nil {
		return nil, fmt.Errorf("%v is not a known struct", rtype)
	}

	vtype := reflect.ValueOf(ctr)
	meta, ok := s.(*reflectplus.Struct)
	if !ok {
		return nil, fmt.Errorf("%v must be a struct but is %v", rtype, reflect.TypeOf(s))
	}

	stereotypeCtr := meta.GetAnnotations().FindFirst(AnnotationStereotypeController)
	oaiGroupTag := ""
	if stereotypeCtr != nil {
		oaiGroupTag = stereotypeCtr.Value()
	}

	prefixRoutes := httpRoutes(meta.Annotations)

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

		var refFunc reflect.Value
		for i := 0; i < vtype.NumMethod(); i++ {
			if vtype.Type().Method(i).Name == method.Name {
				refFunc = vtype.Method(i)
				break
			}
		}
		if !refFunc.IsValid() {
			panic("reflectplus data does not match actual reflect data: method '" + method.Name + "' not found")
		}

		methodParams, err := scanMethodParams(*meta, method)
		if err != nil {
			return nil, reflectplus.PositionalError(method, err)
		}

		for _, prefixRoute := range prefixRoutes {
			for _, route := range routes {
				for _, verb := range verbs {
					path := joinPaths(prefixRoute, route)
					oasPath := pathVarsToOASPath(path)

					item := newPathDoc(res.doc, verb, path,oaiGroupTag, method, methodParams)

					res.doc.Paths[oasPath] = item


					fmt.Printf("registered route %s %s by %s \n", method.Name, path, reflectplus.PositionalError(method, nil).Error())
					srv.handle(verb, path, func(writer http.ResponseWriter, request *http.Request, params KeyValues) error {
						return routedFunc(method, refFunc, methodParams, writer, request, params)
					})

				}
			}
		}

	}

	return res, nil
}

func routedFunc(method reflectplus.Method, refFunc reflect.Value, methodParams []methodParam, writer http.ResponseWriter, request *http.Request, params KeyValues) error {
	args := make([]reflect.Value, 0, len(method.Params))

	fmt.Println(method.Name)
	for _, p := range methodParams {
		switch p.paramType {
		case ptCtx:
			args = append(args, reflect.ValueOf(request.Context()))
		case ptPath:
			parsedType, err := scanToType(params.ByName(p.Alias()), p.param.Type, refFunc.Type().In(p.idx))
			if err != nil {
				return err
			}
			args = append(args, reflect.ValueOf(parsedType))
		case ptQuery:
			url := request.URL
			strValue := url.Query().Get(p.Alias())
			parsedType, err := scanToType(strValue, p.param.Type, refFunc.Type().In(p.idx))
			if err != nil {
				return err
			}
			args = append(args, reflect.ValueOf(parsedType))

		case ptHeader:
			strValue := request.Header.Get(p.Alias())
			parsedType, err := scanToType(strValue, p.param.Type, refFunc.Type().In(p.idx))
			if err != nil {
				return err
			}
			args = append(args, reflect.ValueOf(parsedType))
		case ptRequest:
			args = append(args, reflect.ValueOf(request))
		case ptResponseWriter:
			args = append(args, reflect.ValueOf(writer))
		default:
			panic("method parameter type " + strconv.Itoa(int(p.paramType)) + " not implemented")
		}

	}
	res := refFunc.Call(args)
	for _, v := range res {
		if err, ok := v.Interface().(error); ok {
			return err
		}
	}
	// TODO how to process serialization responses?
	for _, v := range res {
		val := v.Interface()
		if val == nil {
			continue
		}
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		writer.Header().Add("Content-Type", "application/json")
		_, err = writer.Write(b)
		if err != nil {
			return err
		}
	}

	return nil
}

func scanToType(src string, dst reflectplus.TypeDecl, dstType reflect.Type) (interface{}, error) {
	if dst.ImportPath == "" {
		switch dst.Identifier {
		case "int":
			i, err := strconv.ParseInt(src, 10, 64)
			return int(i), err
		case "int64":
			i, err := strconv.ParseInt(src, 10, 64)
			return int64(i), err
		case "int32":
			i, err := strconv.ParseInt(src, 10, 64)
			return int32(i), err
		case "byte":
			i, err := strconv.ParseInt(src, 10, 64)
			return byte(i), err
		case "string":
			return src, nil
		case "float64":
			i, err := strconv.ParseFloat(src, 64)
			return i, err
		case "bool":
			i, err := strconv.ParseBool(src)
			return i, err
		default:
			return nil, fmt.Errorf("scanToType: base type not supported: '%s' (%s)", dst.Identifier, dstType.String())
		}
	}
	val := reflect.New(dstType)
	obj := val.Interface()
	if scanner, ok := obj.(sql.Scanner); ok {
		err := scanner.Scan(src)
		if dstType.Kind() != reflect.Ptr {
			obj = val.Elem().Interface()
		}
		return obj, err
	}
	return nil, fmt.Errorf("unsupported type does not implement http.Scanner: %s", dstType.String())
}

func joinPaths(a, b string) string {
	if a == "" && b == "" {
		return "/"
	}

	if a == "" {
		return b
	}

	if b == "" {
		return a
	}

	sb := &strings.Builder{}
	if !strings.HasPrefix(a, "/") {
		sb.WriteString("/")
	}
	sb.WriteString(a)
	if !strings.HasSuffix(a, "/") {
		if strings.HasPrefix(b, "/") {
			sb.WriteString(b)
		} else {
			sb.WriteString(b[1:])
		}
	}
	p := sb.String()
	if strings.HasSuffix(p, "/") {
		return p[:len(p)-1]
	}
	return p
}

func httpMethods(annotations []reflectplus.Annotation) []string {
	var res []string
	for _, a := range annotations {
		if a.Name == AnnotationMethod {
			method := a.AsString("value")
			if len(method) > 0 {
				res = append(res, method)
			}
		}
	}
	return res
}

func httpRoutes(annotations []reflectplus.Annotation) []string {
	var res []string
	for _, a := range annotations {
		if a.Name == AnnotationRoute {
			v := a.AsString("value")
			if len(v) > 0 {
				res = append(res, v)
			}
		}
	}
	return res
}
