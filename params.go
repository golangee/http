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
	"github.com/worldiety/reflectplus"
	"regexp"
)



var regexParamNames = regexp.MustCompile(":\\w+")

type paramType int

const (
	ptUnknown paramType = 0
	ptCtx               = 1
	ptPath              = 2
	ptQuery             = 3
	ptHeader            = 4
	ptForm              = 5
	ptBody              = 6
)

type methodParam struct {
	paramType paramType
	idx       int
	param     reflectplus.Param
	alias     string
}

func (m methodParam) Alias() string {
	if len(m.alias) > 0 {
		return m.alias
	}
	return m.param.Name
}

// scanMethodParams validates the annotated method and returns unified meta data about the kind of input
func scanMethodParams(parent reflectplus.Struct, method reflectplus.Method) ([]methodParam, error) {
	paramsToDefine := map[string]methodParam{}
	for idx, p := range method.Params {
		paramsToDefine[p.Name] = methodParam{
			idx:   idx,
			param: p,
		}
	}

	var res []methodParam

	// pick up the context
	for _, p := range method.Params {
		if p.Type.ImportPath == "context" && p.Type.Identifier == "Context" {
			tmp := paramsToDefine[p.Name]
			tmp.paramType = ptCtx
			res = append(res, tmp)
			delete(paramsToDefine, p.Name)
		}
	}

	// collect prefix route variables from parent
	for _, p := range parent.FindAnnotations(AnnotationRoute) {
		// collect postfix route variables from method
		for _, a := range method.FindAnnotations(AnnotationRoute) {
			actualRoute := joinPaths(p.Value(), a.Value())
			if len(actualRoute) == 0 {
				return nil, fmt.Errorf("method has an empty route")
			}

			for _, routeParam := range paramNamesFromRoute(actualRoute) {
				if _, has := paramsToDefine[routeParam]; !has {
					return nil, fmt.Errorf("the named route variable '%s' has no matching method parameter", routeParam)
				}
				tmp := paramsToDefine[routeParam]
				tmp.paramType = ptPath
				res = append(res, tmp)
				delete(paramsToDefine, routeParam)
			}
		}
	}

	// collect query params
	for _, a := range method.FindAnnotations(AnnotationQueryParam) {
		name := a.Value()
		if len(name) == 0 {
			return nil, fmt.Errorf("value of '%s' must not be empty", AnnotationQueryParam)
		}

		if _, has := paramsToDefine[name]; !has {
			return nil, fmt.Errorf("the query parameter '%s' has no matching method parameter", name)
		}

		tmp := paramsToDefine[name]
		tmp.alias = a.AsString("alias")
		tmp.paramType = ptQuery
		res = append(res, tmp)
		delete(paramsToDefine, name)
	}

	// collect header params
	for _, a := range method.FindAnnotations(AnnotationHeaderParam) {
		name := a.Value()
		if len(name) == 0 {
			return nil, fmt.Errorf("value of '%s' must not be empty", AnnotationHeaderParam)
		}

		if _, has := paramsToDefine[name]; !has {
			return nil, fmt.Errorf("the header parameter '%s' has no matching method parameter", name)
		}

		tmp := paramsToDefine[name]
		tmp.alias = a.AsString("alias")
		tmp.paramType = ptHeader
		res = append(res, tmp)
		delete(paramsToDefine, name)
	}

	// check for parameters, which have not been defined yet
	for _, p := range paramsToDefine {
		return nil, fmt.Errorf("method parameter '%s' has not been mapped to a request parameter", p.param.Name)
	}

	return res, nil
}

func paramNamesFromRoute(route string) []string {
	names := regexParamNames.FindAllString(string(route), -1)
	for i, n := range names {
		names[i] = n[1:]
	}
	return names
}
