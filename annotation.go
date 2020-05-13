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

// AnnotationQueryParam only applies to method parameters and uses the names as the url query parameter. value' denotes
// the method variable name and 'alias' an optional name (e.g. accept-language)
const AnnotationQueryParam = "ee.http.QueryParam"

// AnnotationHeaderParam only applies to method parameters and uses the names from the http request. 'value' denotes
// the method variable name and 'alias' an optional name (e.g. accept-language)
const AnnotationHeaderParam = "ee.http.HeaderParam"

// AnnotationMethod only applies to methods and describes which http verb is applied for routing.
const AnnotationMethod = "ee.http.Method"

// AnnotationRoute can be used for a struct and/or struct methods. The value is the route with path variables
// preceded by a : like the following example:
//   /api/v1/sms/:id
//   /resource/:kind1/:kind2
const AnnotationRoute = "ee.http.Route"