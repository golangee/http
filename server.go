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
	"github.com/julienschmidt/httprouter"
	"net/http"
	"strconv"
)

type Server struct {
	routes     *httprouter.Router
	middleware []func(Handler) Handler
}

func NewServer() *Server {
	return &Server{
		routes: httprouter.New(),
	}
}

func (s *Server) Use(middleware func(Handler) Handler) {
	s.middleware = append(s.middleware, middleware)
}

func (s *Server) handle(method, path string, handle Handler) {
	s.routes.Handle(method, path, func(writer http.ResponseWriter, request *http.Request, params httprouter.Params) {
		myHandler := handle
		for i := len(s.middleware) - 1; i >= 0; i-- {
			myHandler = s.middleware[i](myHandler)
		}
		err := myHandler(writer, request, wrapRouterParams(params))
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			fmt.Println("error", err)
		}
	})
}

func (s *Server) Start(port int) error {
	return http.ListenAndServe(":"+strconv.Itoa(port), s.routes)
}

func (s *Server) SetNotFound(handler http.Handler){
	s.routes.NotFound = handler
}

// Handler returns the internal handler
func (s *Server) Handler() http.Handler {
	return s.routes
}
