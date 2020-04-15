/*
 * Copyright 2020 Dgraph Labs, Inc. and Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package custom_logic

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/dgraph-io/dgraph/graphql/e2e/common"
	"github.com/dgraph-io/dgraph/testutil"
	"github.com/stretchr/testify/require"
)

const (
	alphaURL      = "http://localhost:8180/graphql"
	alphaAdminURL = "http://localhost:8180/admin"
	customTypes   = `type MovieDirector @remote {
		 id: ID!
		 name: String!
		 directed: [Movie]
	 }
 
	 type Movie @remote {
		 id: ID!
		 name: String!
		 director: [MovieDirector]
	 }
	 type Continent @remote {
		code: String
		name: String
		countries: [Country]
	  }
	  
	  type Country @remote {
		code: String
		name: String
		native: String
		phone: String
		continent: Continent
		currency: String
		languages: [Language]
		emoji: String
		emojiU: String
		states: [State]
	  }
	  
	  type Language @remote {
		code: String
		name: String
		native: String
		rtl: Int
	  }
	  
	  
	  type State @remote {
		code: String
		name: String
		country: Country
	  }
 `
)

func updateSchema(t *testing.T, sch string) *common.GraphQLResponse {
	add := &common.GraphQLParams{
		Query: `mutation updateGQLSchema($sch: String!) {
			 updateGQLSchema(input: { set: { schema: $sch }}) {
				 gqlSchema {
					 schema
				 }
			 }
		 }`,
		Variables: map[string]interface{}{"sch": sch},
	}
	return add.ExecuteAsPost(t, alphaAdminURL)
}

func TestCustomGetQuery(t *testing.T) {
	schema := customTypes + `
	 type Query {
		 myFavoriteMovies(id: ID!, name: String!, num: Int): [Movie] @custom(http: {
				 url: "http://mock:8888/favMovies/$id?name=$name&num=$num",
				 method: "GET"
		 })
	 }`
	common.RequireNoGQLErrors(t, updateSchema(t, schema))

	query := `
	 query {
		 myFavoriteMovies(id: "0x123", name: "Author", num: 10) {
			 id
			 name
			 director {
				 id
				 name
			 }
		 }
	 }`
	params := &common.GraphQLParams{
		Query: query,
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)

	expected := `{"myFavoriteMovies":[{"id":"0x3","name":"Star Wars","director":[{"id":"0x4","name":"George Lucas"}]},{"id":"0x5","name":"Star Trek","director":[{"id":"0x6","name":"J.J. Abrams"}]}]}`
	require.JSONEq(t, expected, string(result.Data))
}

func TestCustomPostQuery(t *testing.T) {
	schema := customTypes + `
	 type Query {
		 myFavoriteMoviesPost(id: ID!, name: String!, num: Int): [Movie] @custom(http: {
				 url: "http://mock:8888/favMoviesPost/$id?name=$name&num=$num",
				 method: "POST"
		 })
	 }`
	common.RequireNoGQLErrors(t, updateSchema(t, schema))

	query := `
	 query {
		 myFavoriteMoviesPost(id: "0x123", name: "Author", num: 10) {
			 id
			 name
			 director {
				 id
				 name
			 }
		 }
	 }`
	params := &common.GraphQLParams{
		Query: query,
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)

	expected := `{"myFavoriteMoviesPost":[{"id":"0x3","name":"Star Wars","director":[{"id":"0x4","name":"George Lucas"}]},{"id":"0x5","name":"Star Trek","director":[{"id":"0x6","name":"J.J. Abrams"}]}]}`
	require.JSONEq(t, expected, string(result.Data))
}

func TestCustomQueryShouldForwardHeaders(t *testing.T) {
	schema := customTypes + `
	 type Query {
		 verifyHeaders(id: ID!): [Movie] @custom(http: {
				 url: "http://mock:8888/verifyHeaders",
				 method: "GET",
				 forwardHeaders: ["X-App-Token", "X-User-Id"]
		 })
	 }`
	common.RequireNoGQLErrors(t, updateSchema(t, schema))

	query := `
	 query {
		 verifyHeaders(id: "0x123") {
			 id
			 name
		 }
	 }`
	params := &common.GraphQLParams{
		Query: query,
		Headers: map[string][]string{
			"X-App-Token":   []string{"app-token"},
			"X-User-Id":     []string{"123"},
			"Random-header": []string{"random"},
		},
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)
	expected := `{"verifyHeaders":[{"id":"0x3","name":"Star Wars"}]}`
	require.Equal(t, expected, string(result.Data))
}

func TestServerShouldAllowForwardHeaders(t *testing.T) {
	schema := `
	type User {
		id: ID!
		name: String!
	}
	type Movie @remote {
		id: ID!
		name: String! @custom(http: {
			url: "http://mock:8888/movieName",
			method: "POST",
			forwardHeaders: ["X-App-User", "X-Group-Id"]
		})
		director: [User] @custom(http: {
			url: "http://mock:8888/movieName",
			method: "POST",
			forwardHeaders: ["User-Id", "X-App-Token"]
		})
	}

	type Query {
		verifyHeaders(id: ID!): [Movie] @custom(http: {
				url: "http://mock:8888/verifyHeaders",
				method: "GET",
				forwardHeaders: ["X-App-Token", "X-User-Id"]
		})
	}`

	updateSchema(t, schema)

	req, err := http.NewRequest(http.MethodOptions, alphaURL, nil)
	require.NoError(t, err)

	resp, err := (&http.Client{}).Do(req)
	require.NoError(t, err)

	headers := strings.Split(resp.Header.Get("Access-Control-Allow-Headers"), ",")
	require.Subset(t, headers, []string{"X-App-Token", "X-User-Id", "User-Id", "X-App-User", "X-Group-Id"})
}

type teacher struct {
	ID  string `json:"tid,omitempty"`
	Age int
}

func addTeachers(t *testing.T) []*teacher {
	addTeacherParams := &common.GraphQLParams{
		Query: `mutation {
			addTeacher(input: [{ age: 28 }, { age: 27 }, { age: 26 }]) {
				teacher {
					tid
					age
				}
			}
		}`,
	}

	result := addTeacherParams.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)

	var res struct {
		AddTeacher struct {
			Teacher []*teacher
		}
	}
	err := json.Unmarshal([]byte(result.Data), &res)
	require.NoError(t, err)

	require.Equal(t, len(res.AddTeacher.Teacher), 3)

	// sort in descending order
	sort.Slice(res.AddTeacher.Teacher, func(i, j int) bool {
		return res.AddTeacher.Teacher[i].Age > res.AddTeacher.Teacher[j].Age
	})
	return res.AddTeacher.Teacher
}

type school struct {
	ID          string `json:"id,omitempty"`
	Established int
}

func addSchools(t *testing.T, teachers []*teacher) []*school {

	params := &common.GraphQLParams{
		Query: `mutation addSchool($t1: [TeacherRef], $t2: [TeacherRef], $t3: [TeacherRef]) {
			 addSchool(input: [{ established: 1980, teachers: $t1 },
				 { established: 1981, teachers: $t2 }, { established: 1982, teachers: $t3 }]) {
				 school {
					 id
					 established
				 }
			 }
		 }`,
		Variables: map[string]interface{}{
			// teachers work at multiple schools.
			"t1": []map[string]interface{}{{"tid": teachers[0].ID}, {"tid": teachers[1].ID}},
			"t2": []map[string]interface{}{{"tid": teachers[1].ID}, {"tid": teachers[2].ID}},
			"t3": []map[string]interface{}{{"tid": teachers[2].ID}, {"tid": teachers[0].ID}},
		},
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nilf(t, result.Errors, "%+v", result.Errors)

	var res struct {
		AddSchool struct {
			School []*school
		}
	}
	err := json.Unmarshal([]byte(result.Data), &res)
	require.NoError(t, err)

	require.Equal(t, len(res.AddSchool.School), 3)
	// The order of mutation result is not the same as the input order, so we sort and return here.
	sort.Slice(res.AddSchool.School, func(i, j int) bool {
		return res.AddSchool.School[i].Established < res.AddSchool.School[j].Established
	})
	return res.AddSchool.School
}

type user struct {
	ID  string `json:"id,omitempty"`
	Age int    `json:"age,omitempty"`
}

func addUsers(t *testing.T, schools []*school) []*user {
	params := &common.GraphQLParams{
		Query: `mutation addUser($s1: [SchoolRef], $s2: [SchoolRef], $s3: [SchoolRef]) {
			 addUser(input: [{ age: 10, schools: $s1 },
				 { age: 11, schools: $s2 }, { age: 12, schools: $s3 }]) {
				 user {
					 id
					 age
				 }
			 }
		 }`,
		Variables: map[string]interface{}{
			// Users could have gone to multiple schools
			"s1": []map[string]interface{}{{"id": schools[0].ID}, {"id": schools[1].ID}},
			"s2": []map[string]interface{}{{"id": schools[1].ID}, {"id": schools[2].ID}},
			"s3": []map[string]interface{}{{"id": schools[2].ID}, {"id": schools[0].ID}},
		},
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nilf(t, result.Errors, "%+v", result.Errors)

	var res struct {
		AddUser struct {
			User []*user
		}
	}
	err := json.Unmarshal([]byte(result.Data), &res)
	require.NoError(t, err)

	require.Equal(t, len(res.AddUser.User), 3)
	// The order of mutation result is not the same as the input order, so we sort and return users here.
	sort.Slice(res.AddUser.User, func(i, j int) bool {
		return res.AddUser.User[i].Age < res.AddUser.User[j].Age
	})
	return res.AddUser.User
}

func verifyData(t *testing.T, users []*user, teachers []*teacher, schools []*school) {
	queryUser := `
	 query {
		 queryUser(order: {asc: age}) {
			 name
			 age
			 cars {
				 name
			 }
			 schools(order: {asc: established}) {
				 name
				 established
				 teachers(order: {desc: age}) {
					 name
					 age
				 }
				 classes {
					 name
				 }
			 }
		 }
	 }`
	params := &common.GraphQLParams{
		Query: queryUser,
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)

	expected := `{
		 "queryUser": [
		   {
			 "name": "uname-` + users[0].ID + `",
			 "age": 10,
			 "cars": {
				 "name": "car-` + users[0].ID + `"
			 },
			 "schools": [
				 {
					 "name": "sname-` + schools[0].ID + `",
					 "established": 1980,
					 "teachers": [
						 {
							 "name": "tname-` + teachers[0].ID + `",
							 "age": 28
						 },
						 {
							 "name": "tname-` + teachers[1].ID + `",
							 "age": 27
						 }
					 ],
					 "classes": [
						 {
							 "name": "class-` + schools[0].ID + `"
						 }
					 ]
				 },
				 {
					 "name": "sname-` + schools[1].ID + `",
					 "established": 1981,
					 "teachers": [
						 {
							 "name": "tname-` + teachers[1].ID + `",
							 "age": 27
						 },
						 {
							 "name": "tname-` + teachers[2].ID + `",
							 "age": 26
						 }
					 ],
					 "classes": [
						 {
							 "name": "class-` + schools[1].ID + `"
						 }
					 ]
				 }
			 ]
		   },
		   {
			 "name": "uname-` + users[1].ID + `",
			 "age": 11,
			 "cars": {
				 "name": "car-` + users[1].ID + `"
			 },
			 "schools": [
				 {
					 "name": "sname-` + schools[1].ID + `",
					 "established": 1981,
					 "teachers": [
						 {
							 "name": "tname-` + teachers[1].ID + `",
							 "age": 27
						 },
						 {
							 "name": "tname-` + teachers[2].ID + `",
							 "age": 26
						 }
					 ],
					 "classes": [
						 {
							 "name": "class-` + schools[1].ID + `"
						 }
					 ]
				 },
				 {
					 "name": "sname-` + schools[2].ID + `",
					 "established": 1982,
					 "teachers": [
						 {
							 "name": "tname-` + teachers[0].ID + `",
							 "age": 28
						 },
						 {
							 "name": "tname-` + teachers[2].ID + `",
							 "age": 26
						 }
					 ],
					 "classes": [
						 {
							 "name": "class-` + schools[2].ID + `"
						 }
					 ]
				 }
			 ]
		   },
		   {
			 "name": "uname-` + users[2].ID + `",
			 "age": 12,
			 "cars": {
				 "name": "car-` + users[2].ID + `"
			 },
			 "schools": [
				 {
					 "name": "sname-` + schools[0].ID + `",
					 "established": 1980,
					 "teachers": [
						 {
							 "name": "tname-` + teachers[0].ID + `",
							 "age": 28
						 },
						 {
							 "name": "tname-` + teachers[1].ID + `",
							 "age": 27
						 }
					 ],
					 "classes": [
						 {
							 "name": "class-` + schools[0].ID + `"
						 }
					 ]
				 },
				 {
					 "name": "sname-` + schools[2].ID + `",
					 "established": 1982,
					 "teachers": [
						 {
							 "name": "tname-` + teachers[0].ID + `",
							 "age": 28
						 },
						 {
							 "name": "tname-` + teachers[2].ID + `",
							 "age": 26
						 }
					 ],
					 "classes": [
						 {
							 "name": "class-` + schools[2].ID + `"
						 }
					 ]
				 }
			 ]
		   }
		 ]
	   }`

	testutil.CompareJSON(t, expected, string(result.Data))

	singleUserQuery := `
	 query {
		 getUser(id: "` + users[0].ID + `") {
			 name
			 age
			 cars {
				 name
			 }
			 schools(order: {asc: established}) {
				 name
				 established
				 teachers(order: {desc: age}) {
					 name
					 age
				 }
				 classes {
					 name
				 }
			 }
		 }
	 }`
	params = &common.GraphQLParams{
		Query: singleUserQuery,
	}

	result = params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)

	expected = `{
		 "getUser": {
			 "name": "uname-` + users[0].ID + `",
			 "age": 10,
			 "cars": {
				 "name": "car-` + users[0].ID + `"
			 },
			 "schools": [
				 {
					 "name": "sname-` + schools[0].ID + `",
					 "established": 1980,
					 "teachers": [
						 {
							 "name": "tname-` + teachers[0].ID + `",
							 "age": 28
						 },
						 {
							 "name": "tname-` + teachers[1].ID + `",
							 "age": 27
						 }
					 ],
					 "classes": [
						 {
							 "name": "class-` + schools[0].ID + `"
						 }
					 ]
				 },
				 {
					 "name": "sname-` + schools[1].ID + `",
					 "established": 1981,
					 "teachers": [
						 {
							 "name": "tname-` + teachers[1].ID + `",
							 "age": 27
						 },
						 {
							 "name": "tname-` + teachers[2].ID + `",
							 "age": 26
						 }
					 ],
					 "classes": [
						 {
							 "name": "class-` + schools[1].ID + `"
						 }
					 ]
				 }
			 ]
		 }
	 }`

	testutil.CompareJSON(t, expected, string(result.Data))

}

func TestCustomFieldsShouldBeResolved(t *testing.T) {
	// lets check batch mode first
	schema := `type Car @remote {
		 id: ID!
		 name: String!
	 }
 
	 type User {
		 id: ID!
		 name: String @custom(http: {
						 url: "http://mock:8888/userNames",
						 method: "GET",
						 body: "{uid: $id}",
						 operation: "batch"
					 })
		 age: Int! @search
		 cars: Car @custom(http: {
						 url: "http://mock:8888/cars",
						 method: "GET",
						 body: "{uid: $id}",
						 operation: "batch"
					 })
		 schools: [School]
	 }
 
	 type School {
		 id: ID!
		 established: Int! @search
		 name: String @custom(http: {
						 url: "http://mock:8888/schoolNames",
						 method: "POST",
						 body: "{sid: $id}",
						 operation: "batch"
					   })
		 classes: [Class] @custom(http: {
							 url: "http://mock:8888/classes",
							 method: "POST",
							 body: "{sid: $id}",
							 operation: "batch"
						 })
		 teachers: [Teacher]
	 }
 
	 type Class @remote {
		 id: ID!
		 name: String!
	 }
 
	 type Teacher {
		 tid: ID!
		 age: Int!
		 name: String @custom(http: {
						 url: "http://mock:8888/teacherNames",
						 method: "POST",
						 body: "{tid: $tid}",
						 operation: "batch"
					 })
	 }`

	common.RequireNoGQLErrors(t, updateSchema(t, schema))

	teachers := addTeachers(t)
	schools := addSchools(t, teachers)
	users := addUsers(t, schools)

	verifyData(t, users, teachers, schools)

	// lets update the schema and check single mode now
	schema = `
	 type Car @remote {
		 id: ID!
		 name: String!
	 }
 
	 type User {
		 id: ID!
		 name: String @custom(http: {
						 url: "http://mock:8888/userName",
						 method: "GET",
						 body: "{uid: $id}",
						 operation: "single"
					 })
		 age: Int! @search
		 cars: Car @custom(http: {
						 url: "http://mock:8888/car",
						 method: "GET",
						 body: "{uid: $id}",
						 operation: "single"
					 })
		 schools: [School]
	 }
 
	 type School {
		 id: ID!
		 established: Int! @search
		 name: String @custom(http: {
						 url: "http://mock:8888/schoolName",
						 method: "POST",
						 body: "{sid: $id}",
						 operation: "single"
					   })
		 classes: [Class] @custom(http: {
							 url: "http://mock:8888/class",
							 method: "POST",
							 body: "{sid: $id}",
							 operation: "single"
						 })
		 teachers: [Teacher]
	 }
 
	 type Class @remote {
		 id: ID!
		 name: String!
	 }
 
	 type Teacher {
		 tid: ID!
		 age: Int!
		 name: String @custom(http: {
						 url: "http://mock:8888/teacherName",
						 method: "POST",
						 body: "{tid: $tid}",
						 operation: "single"
					   })
	 }`

	verifyData(t, users, teachers, schools)
}

func TestForInvalidCustomQuery(t *testing.T) {
	schema := customTypes + `
	type Query {
		getCountry(id: ID!): Country! @custom(http: {url: "http://mock:8888/noquery", method: "POST",forwardHeaders: ["Content-Type"]}, graphql: {query: "country(code: $id)"})
	}	
	`
	res := updateSchema(t, schema)
	require.Equal(t, res.Errors[0].Error(), "couldn't rewrite mutation updateGQLSchema because input:46: Type Query; Field getCountry; country is not present in remote schema\n")
}

func TestForInvalidArguement(t *testing.T) {
	schema := customTypes + `
	type Query {
		getCountry(id: ID!): Country! @custom(http: {url: "http://mock:8888/invalidargument", method: "POST",forwardHeaders: ["Content-Type"]}, graphql: {query: "country(code: $id)"})
	}	
	`
	res := updateSchema(t, schema)
	require.Equal(t, res.Errors[0].Error(), "couldn't rewrite mutation updateGQLSchema because input:46: Type Query; Field getCountry; code arg not present in the remote query country\n")
}

func TestForInvalidType(t *testing.T) {
	schema := customTypes + `
	type Query {
		getCountry(id: ID!): Country! @custom(http: {url: "http://mock:8888/invalidtype", method: "POST",forwardHeaders: ["Content-Type"]}, graphql: {query: "country(code: $id)"})
	}	
	`
	res := updateSchema(t, schema)
	require.Equal(t, res.Errors[0].Error(), "couldn't rewrite mutation updateGQLSchema because input:46: Type Query; Field getCountry; expected type for variable  $id is Int. But got ID!\n")
}

func TestCustomLogicGraphql(t *testing.T) {
	schema := customTypes + `
	type Query {
		getCountry(id: ID!): Country! @custom(http: {url: "http://mock:8888/validcountry", method: "POST"}, graphql: {query: "country(code: $id)"})
	}	
	`
	res := updateSchema(t, schema)
	require.Nil(t, res.Errors)
	query := `
	query {
		getCountry(id: "BI"){
			code
			name 
		}
	}`
	params := &common.GraphQLParams{
		Query: query,
	}

	result := params.ExecuteAsPost(t, alphaURL)
	common.RequireNoGQLErrors(t, result)
	require.JSONEq(t, string(result.Data), `
	{"getCountry":{"code":"BI","name":"Burundi"}}
	`)
}

func TestCustomLogicGraphqlWithError(t *testing.T) {
	schema := customTypes + `
	type Query {
		getCountry(id: ID!): Country! @custom(http: {url: "http://mock:8888/validcountrywitherror", method: "POST"}, graphql: {query: "country(code: $id)"})
	}	
	`
	common.RequireNoGQLErrors(t, updateSchema(t, schema))
	query := `
	query {
		getCountry(id: "BI"){
			code
			name 
		}
	}`
	params := &common.GraphQLParams{
		Query: query,
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.JSONEq(t, string(result.Data), `
	{"getCountry":{"code":"BI","name":"Burundi"}}
	`)
	require.Equal(t, "dummy error", result.Errors.Error())
}

// func TestCustomLogicGraphqlValidSlice(t *testing.T) {
// 	schema := customTypes + `
// 	type Query {
// 		getCountry(id: ID!): [Country] @custom(http: {url: "http://mock:8888/validcountries", method: "POST"}, graphql: {query: "country(code: $id)"})
// 	}
// 	`
// 	common.RequireNoGQLErrors(t, updateSchema(t, schema))
// 	query := `
// 	query {
// 		getCountry(id: "BI"){
// 			code
// 			name
// 		}
// 	}`
// 	params := &common.GraphQLParams{
// 		Query: query,
// 	}

// 	result := params.ExecuteAsPost(t, alphaURL)
// 	fmt.Println(string(result.Data))
// 	require.JSONEq(t, string(result.Data), `
// 	{"getCountry":[
// 		{
// 		  "name": "Burundi",
// 		  "code": "BI"
// 		}
// 	  ]}
// 	`)
// }

// func TestCustomLogicWithErrorResponse(t *testing.T) {
// 	schema := customTypes + `
// 	type Query {
// 		getCountry(id: ID!): [Country] @custom(http: {url: "http://mock:8888/graphqlerr", method: "POST"}, graphql: {query: "country(code: $id)"})
// 	}
// 	`
// 	common.RequireNoGQLErrors(t, updateSchema(t, schema))
// 	query := `
// 	query {
// 		getCountry(id: "BI"){
// 			code
// 			name
// 		}
// 	}`
// 	params := &common.GraphQLParams{
// 		Query: query,
// 	}

// 	result := params.ExecuteAsPost(t, alphaURL)
// 	require.Equal(t, "dummy error", result.Errors.Error())
// }

type episode struct {
	Name string
}

func addEpisode(t *testing.T, name string) {
	params := &common.GraphQLParams{
		Query: `mutation addEpisode($name: String!) {
			addEpisode(input: [{ name: $name }]) {
				episode {
					name
				}
			}
		}`,
		Variables: map[string]interface{}{
			"name": name,
		},
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)

	var res struct {
		AddEpisode struct {
			Episode []*episode
		}
	}
	err := json.Unmarshal([]byte(result.Data), &res)
	require.NoError(t, err)

	require.Equal(t, len(res.AddEpisode.Episode), 1)
}

type character struct {
	Name string
}

func addCharacter(t *testing.T, name string, episodes interface{}) {
	params := &common.GraphQLParams{
		Query: `mutation addCharacter($name: String!, $episodes: [EpisodeRef]) {
			addCharacter(input: [{ name: $name, episodes: $episodes }]) {
				character {
					name
					episodes {
						name
					}
				}
			}
		}`,
		Variables: map[string]interface{}{
			"name":     name,
			"episodes": episodes,
		},
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)

	var res struct {
		AddCharacter struct {
			Character []*character
		}
	}
	err := json.Unmarshal([]byte(result.Data), &res)
	require.NoError(t, err)

	require.Equal(t, len(res.AddCharacter.Character), 1)
}

func TestCustomFieldsWithXidShouldBeResolved(t *testing.T) {
	schema := `
	type Episode {
		name: String! @id
		anotherName: String! @custom(http: {
					url: "http://mock:8888/userNames",
					method: "GET",
					body: "{uid: $name}",
					operation: "batch"
				})
	}

	type Character {
		name: String! @id
		lastName: String @custom(http: {
						url: "http://mock:8888/userNames",
						method: "GET",
						body: "{uid: $name}",
						operation: "batch"
					})
		episodes: [Episode]
	}`
	updateSchema(t, schema)

	ep1 := "episode-1"
	ep2 := "episode-2"
	ep3 := "episode-3"

	addEpisode(t, ep1)
	addEpisode(t, ep2)
	addEpisode(t, ep3)

	addCharacter(t, "character-1", []map[string]interface{}{{"name": ep1}, {"name": ep2}})
	addCharacter(t, "character-2", []map[string]interface{}{{"name": ep2}, {"name": ep3}})
	addCharacter(t, "character-3", []map[string]interface{}{{"name": ep3}, {"name": ep1}})

	queryCharacter := `
	query {
		queryCharacter {
			name
			lastName
			episodes {
				name
				anotherName
			}
		}
	}`
	params := &common.GraphQLParams{
		Query: queryCharacter,
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)

	expected := `{
		"queryCharacter": [
		  {
			"name": "character-1",
			"lastName": "uname-character-1",
			"episodes": [
			  {
				"name": "episode-1",
				"anotherName": "uname-episode-1"
			  },
			  {
				"name": "episode-2",
				"anotherName": "uname-episode-2"
			  }
			]
		  },
		  {
			"name": "character-2",
			"lastName": "uname-character-2",
			"episodes": [
			  {
				"name": "episode-2",
				"anotherName": "uname-episode-2"
			  },
			  {
				"name": "episode-3",
				"anotherName": "uname-episode-3"
			  }
			]
		  },
		  {
			"name": "character-3",
			"lastName": "uname-character-3",
			"episodes": [
			  {
				"name": "episode-1",
				"anotherName": "uname-episode-1"
			  },
			  {
				"name": "episode-3",
				"anotherName": "uname-episode-3"
			  }
			]
		  }
		]
	  }`

	testutil.CompareJSON(t, expected, string(result.Data))

	// In this case the types have ID! field but it is not being requested as part of the query
	// explicitly, so custom logic de-duplication should check for "dgraph-uid" field.
	schema = `
	type Episode {
		id: ID!
		name: String! @id
		anotherName: String! @custom(http: {
					url: "http://mock:8888/userNames",
					method: "GET",
					body: "{uid: $name}",
					operation: "batch"
				})
	}

	type Character {
		id: ID!
		name: String! @id
		lastName: String @custom(http: {
						url: "http://mock:8888/userNames",
						method: "GET",
						body: "{uid: $name}",
						operation: "batch"
					})
		episodes: [Episode]
	}`
	updateSchema(t, schema)

	result = params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)
	testutil.CompareJSON(t, expected, string(result.Data))

}

func TestCustomPostMutation(t *testing.T) {
	schema := customTypes + `
	input MovieDirectorInput {
		id: ID
		name: String
		directed: [MovieInput]
	}
	input MovieInput {
		id: ID
		name: String
		director: [MovieDirectorInput]
	}
	type Mutation {
        createMyFavouriteMovies(input: [MovieInput!]): [Movie] @custom(http: {
			url: "http://mock:8888/favMoviesCreate",
			method: "POST",
			body: "{ movies: $input}"
        })
	}`
	updateSchema(t, schema)

	params := &common.GraphQLParams{
		Query: `
		mutation createMovies($movs: [MovieInput!]) {
			createMyFavouriteMovies(input: $movs) {
				id
				name
				director {
					id
					name
				}
			}
		}`,
		Variables: map[string]interface{}{
			"movs": []interface{}{
				map[string]interface{}{
					"name":     "Mov1",
					"director": []interface{}{map[string]interface{}{"name": "Dir1"}},
				},
				map[string]interface{}{"name": "Mov2"},
			}},
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)

	expected := `
	{
      "createMyFavouriteMovies": [
        {
          "id": "0x1",
          "name": "Mov1",
          "director": [
            {
              "id": "0x2",
              "name": "Dir1"
            }
          ]
        },
        {
          "id": "0x3",
          "name": "Mov2",
          "director": []
        }
      ]
    }`
	require.JSONEq(t, expected, string(result.Data))
}

func TestCustomPatchMutation(t *testing.T) {
	schema := customTypes + `
	input MovieDirectorInput {
		id: ID
		name: String
		directed: [MovieInput]
	}
	input MovieInput {
		id: ID
		name: String
		director: [MovieDirectorInput]
	}
	type Mutation {
        updateMyFavouriteMovie(id: ID!, input: MovieInput!): Movie @custom(http: {
			url: "http://mock:8888/favMoviesUpdate/$id",
			method: "PATCH",
			body: "$input"
        })
	}`
	updateSchema(t, schema)

	params := &common.GraphQLParams{
		Query: `
		mutation updateMovies($id: ID!, $mov: MovieInput!) {
			updateMyFavouriteMovie(id: $id, input: $mov) {
				id
				name
				director {
					id
					name
				}
			}
		}`,
		Variables: map[string]interface{}{
			"id": "0x1",
			"mov": map[string]interface{}{
				"name":     "Mov1",
				"director": []interface{}{map[string]interface{}{"name": "Dir1"}},
			}},
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)

	expected := `
	{
      "updateMyFavouriteMovie": {
        "id": "0x1",
        "name": "Mov1",
        "director": [
          {
            "id": "0x2",
            "name": "Dir1"
          }
        ]
      }
    }`
	require.JSONEq(t, expected, string(result.Data))
}

func TestCustomMutationShouldForwardHeaders(t *testing.T) {
	schema := customTypes + `
	type Mutation {
        deleteMyFavouriteMovie(id: ID!): Movie @custom(http: {
			url: "http://mock:8888/favMoviesDelete/$id",
			method: "DELETE",
			forwardHeaders: ["X-App-Token", "X-User-Id"]
        })
	}`
	updateSchema(t, schema)

	params := &common.GraphQLParams{
		Query: `
		mutation {
			deleteMyFavouriteMovie(id: "0x1") {
				id
				name
			}
		}`,
		Headers: map[string][]string{
			"X-App-Token":   {"app-token"},
			"X-User-Id":     {"123"},
			"Random-header": {"random"},
		},
	}

	result := params.ExecuteAsPost(t, alphaURL)
	require.Nil(t, result.Errors)

	expected := `
	{
      "deleteMyFavouriteMovie": {
        "id": "0x1",
        "name": "Mov1"
      }
    }`
	require.JSONEq(t, expected, string(result.Data))
}