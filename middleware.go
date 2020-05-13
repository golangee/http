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
	"github.com/julienschmidt/httprouter"
	"net/http"
)

type Param struct {
	Key   string
	Value string
}

type Params []Param

func (p Params) ByName(name string) string {
	for i := range p {
		if p[i].Key == name {
			return p[i].Value
		}
	}
	return ""
}

// wrapRouterParams is an implementation abstraction firewall against the used router implementation
func wrapRouterParams(params httprouter.Params) Params {
	r := make([]Param, len(params), len(params))
	for i, p := range params {
		r[i] = Param(p)
	}
	return r
}

type Handler func(writer http.ResponseWriter, request *http.Request, params Params) error
