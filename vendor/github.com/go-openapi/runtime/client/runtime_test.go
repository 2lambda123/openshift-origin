// Copyright 2015 go-swagger maintainers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"crypto/x509"
	"encoding/pem"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// task This describes a task. Tasks require a content property to be set.
type task struct {

	// Completed
	Completed bool `json:"completed" xml:"completed"`

	// Content Task content can contain [GFM](https://help.github.com/articles/github-flavored-markdown/).
	Content string `json:"content" xml:"content"`

	// ID This id property is autogenerated when a task is created.
	ID int64 `json:"id" xml:"id"`
}

func TestRuntime_TLSAuthConfig(t *testing.T) {
	var opts TLSClientOptions
	opts.CA = "../fixtures/certs/myCA.crt"
	opts.Key = "../fixtures/certs/myclient.key"
	opts.Certificate = "../fixtures/certs/myclient.crt"
	opts.ServerName = "somewhere"

	cfg, err := TLSClientAuth(opts)
	if assert.NoError(t, err) {
		if assert.NotNil(t, cfg) {
			assert.Len(t, cfg.Certificates, 1)
			assert.NotNil(t, cfg.RootCAs)
			assert.Equal(t, "somewhere", cfg.ServerName)
		}
	}
}

func TestRuntime_TLSAuthConfigWithRSAKey(t *testing.T) {

	keyPem, err := ioutil.ReadFile("../fixtures/certs/myclient.key")
	require.NoError(t, err)

	keyDer, _ := pem.Decode(keyPem)
	require.NotNil(t, keyDer)

	key, err := x509.ParsePKCS1PrivateKey(keyDer.Bytes)
	require.NoError(t, err)

	certPem, err := ioutil.ReadFile("../fixtures/certs/myclient.crt")
	require.NoError(t, err)

	certDer, _ := pem.Decode(certPem)
	require.NotNil(t, certDer)

	cert, err := x509.ParseCertificate(certDer.Bytes)

	var opts TLSClientOptions
	opts.LoadedKey = key
	opts.LoadedCertificate = cert

	cfg, err := TLSClientAuth(opts)
	if assert.NoError(t, err) {
		if assert.NotNil(t, cfg) {
			assert.Len(t, cfg.Certificates, 1)
		}
	}
}

func TestRuntime_TLSAuthConfigWithECKey(t *testing.T) {

	keyPem, err := ioutil.ReadFile("../fixtures/certs/myclient-ecc.key")
	require.NoError(t, err)

	_, remainder := pem.Decode(keyPem)
	keyDer, _ := pem.Decode(remainder)
	require.NotNil(t, keyDer)

	key, err := x509.ParseECPrivateKey(keyDer.Bytes)
	require.NoError(t, err)

	certPem, err := ioutil.ReadFile("../fixtures/certs/myclient-ecc.crt")
	require.NoError(t, err)

	certDer, _ := pem.Decode(certPem)
	require.NotNil(t, certDer)

	cert, err := x509.ParseCertificate(certDer.Bytes)

	var opts TLSClientOptions
	opts.LoadedKey = key
	opts.LoadedCertificate = cert

	cfg, err := TLSClientAuth(opts)
	if assert.NoError(t, err) {
		if assert.NotNil(t, cfg) {
			assert.Len(t, cfg.Certificates, 1)
		}
	}
}

func TestRuntime_TLSAuthConfigWithLoadedCA(t *testing.T) {

	certPem, err := ioutil.ReadFile("../fixtures/certs/myCA.crt")
	require.NoError(t, err)

	block, _ := pem.Decode(certPem)
	require.NotNil(t, block)

	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	var opts TLSClientOptions
	opts.LoadedCA = cert

	cfg, err := TLSClientAuth(opts)
	if assert.NoError(t, err) {
		if assert.NotNil(t, cfg) {
			assert.NotNil(t, cfg.RootCAs)
		}
	}
}

func TestRuntime_Concurrent(t *testing.T) {
	// test that it can make a simple request
	// and get the response for it.
	// defaults all the way down
	result := []task{
		{false, "task 1 content", 1},
		{false, "task 2 content", 2},
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Add(runtime.HeaderContentType, runtime.JSONMime)
		rw.WriteHeader(http.StatusOK)
		jsongen := json.NewEncoder(rw)
		_ = jsongen.Encode(result)
	}))
	defer server.Close()

	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return nil
	})

	hu, _ := url.Parse(server.URL)
	rt := New(hu.Host, "/", []string{"http"})
	resCC := make(chan interface{})
	errCC := make(chan error)
	var res interface{}
	var err error

	for j := 0; j < 6; j++ {
		go func() {
			resC := make(chan interface{})
			errC := make(chan error)

			go func() {
				var resp interface{}
				var errp error
				for i := 0; i < 3; i++ {
					resp, errp = rt.Submit(&runtime.ClientOperation{
						ID:          "getTasks",
						Method:      "GET",
						PathPattern: "/",
						Params:      rwrtr,
						Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
							if response.Code() == 200 {
								var result []task
								if err := consumer.Consume(response.Body(), &result); err != nil {
									return nil, err
								}
								return result, nil
							}
							return nil, errors.New("Generic error")
						}),
					})
					<-time.After(100 * time.Millisecond)
				}
				resC <- resp
				errC <- errp
			}()
			resCC <- <-resC
			errCC <- <-errC
		}()
	}

	c := 6
	for c > 0 {
		res = <-resCC
		err = <-errCC
		c--
	}

	if assert.NoError(t, err) {
		assert.IsType(t, []task{}, res)
		actual := res.([]task)
		assert.EqualValues(t, result, actual)
	}
}

func TestRuntime_Canary(t *testing.T) {
	// test that it can make a simple request
	// and get the response for it.
	// defaults all the way down
	result := []task{
		{false, "task 1 content", 1},
		{false, "task 2 content", 2},
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Add(runtime.HeaderContentType, runtime.JSONMime)
		rw.WriteHeader(http.StatusOK)
		jsongen := json.NewEncoder(rw)
		_ = jsongen.Encode(result)
	}))
	defer server.Close()

	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return nil
	})

	hu, _ := url.Parse(server.URL)
	rt := New(hu.Host, "/", []string{"http"})
	res, err := rt.Submit(&runtime.ClientOperation{
		ID:          "getTasks",
		Method:      "GET",
		PathPattern: "/",
		Params:      rwrtr,
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
			if response.Code() == 200 {
				var result []task
				if err := consumer.Consume(response.Body(), &result); err != nil {
					return nil, err
				}
				return result, nil
			}
			return nil, errors.New("Generic error")
		}),
	})

	if assert.NoError(t, err) {
		assert.IsType(t, []task{}, res)
		actual := res.([]task)
		assert.EqualValues(t, result, actual)
	}
}

type tasks struct {
	Tasks []task `xml:"task"`
}

func TestRuntime_XMLCanary(t *testing.T) {
	// test that it can make a simple XML request
	// and get the response for it.
	result := tasks{
		Tasks: []task{
			{false, "task 1 content", 1},
			{false, "task 2 content", 2},
		},
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Add(runtime.HeaderContentType, runtime.XMLMime)
		rw.WriteHeader(http.StatusOK)
		xmlgen := xml.NewEncoder(rw)
		_ = xmlgen.Encode(result)
	}))
	defer server.Close()

	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return nil
	})

	hu, _ := url.Parse(server.URL)
	rt := New(hu.Host, "/", []string{"http"})
	res, err := rt.Submit(&runtime.ClientOperation{
		ID:          "getTasks",
		Method:      "GET",
		PathPattern: "/",
		Params:      rwrtr,
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
			if response.Code() == 200 {
				var result tasks
				if err := consumer.Consume(response.Body(), &result); err != nil {
					return nil, err
				}
				return result, nil
			}
			return nil, errors.New("Generic error")
		}),
	})

	if assert.NoError(t, err) {
		assert.IsType(t, tasks{}, res)
		actual := res.(tasks)
		assert.EqualValues(t, result, actual)
	}
}

func TestRuntime_TextCanary(t *testing.T) {
	// test that it can make a simple text request
	// and get the response for it.
	result := "1: task 1 content; 2: task 2 content"
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Add(runtime.HeaderContentType, runtime.TextMime)
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte(result))
	}))
	defer server.Close()

	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return nil
	})

	hu, _ := url.Parse(server.URL)
	rt := New(hu.Host, "/", []string{"http"})
	res, err := rt.Submit(&runtime.ClientOperation{
		ID:          "getTasks",
		Method:      "GET",
		PathPattern: "/",
		Params:      rwrtr,
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
			if response.Code() == 200 {
				var result string
				if err := consumer.Consume(response.Body(), &result); err != nil {
					return nil, err
				}
				return result, nil
			}
			return nil, errors.New("Generic error")
		}),
	})

	if assert.NoError(t, err) {
		assert.IsType(t, "", res)
		actual := res.(string)
		assert.EqualValues(t, result, actual)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRuntime_CustomTransport(t *testing.T) {
	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return nil
	})
	result := []task{
		{false, "task 1 content", 1},
		{false, "task 2 content", 2},
	}

	rt := New("localhost:3245", "/", []string{"ws", "wss", "https"})
	rt.Transport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Scheme != "https" {
			return nil, errors.New("this was not a https request")
		}
		var resp http.Response
		resp.StatusCode = 200
		resp.Header = make(http.Header)
		resp.Header.Set("content-type", "application/json")
		buf := bytes.NewBuffer(nil)
		enc := json.NewEncoder(buf)
		_ = enc.Encode(result)
		resp.Body = ioutil.NopCloser(buf)
		return &resp, nil
	})

	res, err := rt.Submit(&runtime.ClientOperation{
		ID:          "getTasks",
		Method:      "GET",
		PathPattern: "/",
		Schemes:     []string{"ws", "wss", "https"},
		Params:      rwrtr,
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
			if response.Code() == 200 {
				var result []task
				if err := consumer.Consume(response.Body(), &result); err != nil {
					return nil, err
				}
				return result, nil
			}
			return nil, errors.New("Generic error")
		}),
	})

	if assert.NoError(t, err) {
		assert.IsType(t, []task{}, res)
		actual := res.([]task)
		assert.EqualValues(t, result, actual)
	}
}

func TestRuntime_CustomCookieJar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		authenticated := false
		for _, cookie := range req.Cookies() {
			if cookie.Name == "sessionid" && cookie.Value == "abc" {
				authenticated = true
			}
		}
		if !authenticated {
			username, password, ok := req.BasicAuth()
			if ok && username == "username" && password == "password" {
				authenticated = true
				http.SetCookie(rw, &http.Cookie{Name: "sessionid", Value: "abc"})
			}
		}
		if authenticated {
			rw.Header().Add(runtime.HeaderContentType, runtime.JSONMime)
			rw.WriteHeader(http.StatusOK)
			jsongen := json.NewEncoder(rw)
			_ = jsongen.Encode([]task{})
		} else {
			rw.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return nil
	})

	hu, _ := url.Parse(server.URL)
	rt := New(hu.Host, "/", []string{"http"})
	rt.Jar, _ = cookiejar.New(nil)

	submit := func(authInfo runtime.ClientAuthInfoWriter) {
		_, err := rt.Submit(&runtime.ClientOperation{
			ID:          "getTasks",
			Method:      "GET",
			PathPattern: "/",
			Params:      rwrtr,
			AuthInfo:    authInfo,
			Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
				if response.Code() == 200 {
					return nil, nil
				}
				return nil, errors.New("Generic error")
			}),
		})

		assert.NoError(t, err)
	}

	submit(BasicAuth("username", "password"))
	submit(nil)
}

func TestRuntime_AuthCanary(t *testing.T) {
	// test that it can make a simple request
	// and get the response for it.
	// defaults all the way down
	result := []task{
		{false, "task 1 content", 1},
		{false, "task 2 content", 2},
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer the-super-secret-token" {
			rw.WriteHeader(400)
			return
		}
		rw.Header().Add(runtime.HeaderContentType, runtime.JSONMime)
		rw.WriteHeader(http.StatusOK)
		jsongen := json.NewEncoder(rw)
		_ = jsongen.Encode(result)
	}))
	defer server.Close()

	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return nil
	})

	hu, _ := url.Parse(server.URL)

	rt := New(hu.Host, "/", []string{"http"})
	res, err := rt.Submit(&runtime.ClientOperation{
		ID:     "getTasks",
		Params: rwrtr,
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
			if response.Code() == 200 {
				var result []task
				if err := consumer.Consume(response.Body(), &result); err != nil {
					return nil, err
				}
				return result, nil
			}
			return nil, errors.New("Generic error")
		}),
		AuthInfo: BearerToken("the-super-secret-token"),
	})

	if assert.NoError(t, err) {
		assert.IsType(t, []task{}, res)
		actual := res.([]task)
		assert.EqualValues(t, result, actual)
	}
}

func TestRuntime_PickConsumer(t *testing.T) {
	result := []task{
		{false, "task 1 content", 1},
		{false, "task 2 content", 2},
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Content-Type") != "application/octet-stream" {
			rw.Header().Add(runtime.HeaderContentType, runtime.JSONMime+";charset=utf-8")
			rw.WriteHeader(400)
			return
		}
		rw.Header().Add(runtime.HeaderContentType, runtime.JSONMime+";charset=utf-8")
		rw.WriteHeader(http.StatusOK)
		jsongen := json.NewEncoder(rw)
		_ = jsongen.Encode(result)
	}))
	defer server.Close()

	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return req.SetBodyParam(bytes.NewBufferString("hello"))
	})

	hu, _ := url.Parse(server.URL)
	rt := New(hu.Host, "/", []string{"http"})
	res, err := rt.Submit(&runtime.ClientOperation{
		ID:                 "getTasks",
		Method:             "POST",
		PathPattern:        "/",
		Schemes:            []string{"http"},
		ConsumesMediaTypes: []string{"application/octet-stream"},
		Params:             rwrtr,
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
			if response.Code() == 200 {
				var result []task
				if err := consumer.Consume(response.Body(), &result); err != nil {
					return nil, err
				}
				return result, nil
			}
			return nil, errors.New("Generic error")
		}),
		AuthInfo: BearerToken("the-super-secret-token"),
	})

	if assert.NoError(t, err) {
		assert.IsType(t, []task{}, res)
		actual := res.([]task)
		assert.EqualValues(t, result, actual)
	}
}

func TestRuntime_ContentTypeCanary(t *testing.T) {
	// test that it can make a simple request
	// and get the response for it.
	// defaults all the way down
	result := []task{
		{false, "task 1 content", 1},
		{false, "task 2 content", 2},
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer the-super-secret-token" {
			rw.WriteHeader(400)
			return
		}
		rw.Header().Add(runtime.HeaderContentType, runtime.JSONMime+";charset=utf-8")
		rw.WriteHeader(http.StatusOK)
		jsongen := json.NewEncoder(rw)
		_ = jsongen.Encode(result)
	}))
	defer server.Close()

	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return nil
	})

	hu, _ := url.Parse(server.URL)
	rt := New(hu.Host, "/", []string{"http"})
	res, err := rt.Submit(&runtime.ClientOperation{
		ID:          "getTasks",
		Method:      "GET",
		PathPattern: "/",
		Schemes:     []string{"http"},
		Params:      rwrtr,
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
			if response.Code() == 200 {
				var result []task
				if err := consumer.Consume(response.Body(), &result); err != nil {
					return nil, err
				}
				return result, nil
			}
			return nil, errors.New("Generic error")
		}),
		AuthInfo: BearerToken("the-super-secret-token"),
	})

	if assert.NoError(t, err) {
		assert.IsType(t, []task{}, res)
		actual := res.([]task)
		assert.EqualValues(t, result, actual)
	}
}

func TestRuntime_ChunkedResponse(t *testing.T) {
	// test that it can make a simple request
	// and get the response for it.
	// defaults all the way down
	result := []task{
		{false, "task 1 content", 1},
		{false, "task 2 content", 2},
	}
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer the-super-secret-token" {
			rw.WriteHeader(400)
			return
		}
		rw.Header().Add(runtime.HeaderTransferEncoding, "chunked")
		rw.Header().Add(runtime.HeaderContentType, runtime.JSONMime+";charset=utf-8")
		rw.WriteHeader(http.StatusOK)
		jsongen := json.NewEncoder(rw)
		_ = jsongen.Encode(result)
	}))
	defer server.Close()

	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return nil
	})

	//specDoc, err := spec.Load("../../fixtures/codegen/todolist.simple.yml")
	hu, _ := url.Parse(server.URL)

	rt := New(hu.Host, "/", []string{"http"})
	res, err := rt.Submit(&runtime.ClientOperation{
		ID:          "getTasks",
		Method:      "GET",
		PathPattern: "/",
		Schemes:     []string{"http"},
		Params:      rwrtr,
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
			if response.Code() == 200 {
				var result []task
				if err := consumer.Consume(response.Body(), &result); err != nil {
					return nil, err
				}
				return result, nil
			}
			return nil, errors.New("Generic error")
		}),
		AuthInfo: BearerToken("the-super-secret-token"),
	})

	if assert.NoError(t, err) {
		assert.IsType(t, []task{}, res)
		actual := res.([]task)
		assert.EqualValues(t, result, actual)
	}
}

func TestRuntime_DebugValue(t *testing.T) {
	original := os.Getenv("DEBUG")

	// Emtpy DEBUG means Debug is False
	_ = os.Setenv("DEBUG", "")
	runtime := New("", "/", []string{"https"})
	assert.False(t, runtime.Debug)

	// Non-Empty Debug means Debug is True

	_ = os.Setenv("DEBUG", "1")
	runtime = New("", "/", []string{"https"})
	assert.True(t, runtime.Debug)

	_ = os.Setenv("DEBUG", "true")
	runtime = New("", "/", []string{"https"})
	assert.True(t, runtime.Debug)

	_ = os.Setenv("DEBUG", "foo")
	runtime = New("", "/", []string{"https"})
	assert.True(t, runtime.Debug)

	// Make sure DEBUG is initial value once again
	_ = os.Setenv("DEBUG", original)
}

func TestRuntime_OverrideScheme(t *testing.T) {
	runtime := New("", "/", []string{"https"})
	sch := runtime.pickScheme([]string{"http"})
	assert.Equal(t, "https", sch)
}

func TestRuntime_OverrideClient(t *testing.T) {
	client := &http.Client{}
	runtime := NewWithClient("", "/", []string{"https"}, client)
	var i int
	runtime.clientOnce.Do(func() { i++ })
	assert.Equal(t, client, runtime.client)
	assert.Equal(t, 0, i)
}

type overrideRoundTripper struct {
	overriden bool
}

func (o *overrideRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	o.overriden = true
	res := new(http.Response)
	res.StatusCode = 200
	res.Body = ioutil.NopCloser(bytes.NewBufferString("OK"))
	return res, nil
}

func TestRuntime_OverrideClientOperation(t *testing.T) {
	client := &http.Client{}
	rt := NewWithClient("", "/", []string{"https"}, client)
	var i int
	rt.clientOnce.Do(func() { i++ })
	assert.Equal(t, client, rt.client)
	assert.Equal(t, 0, i)

	client2 := new(http.Client)
	var transport = &overrideRoundTripper{}
	client2.Transport = transport
	if assert.NotEqual(t, client, client2) {
		_, err := rt.Submit(&runtime.ClientOperation{
			Client: client2,
			Params: runtime.ClientRequestWriterFunc(func(r runtime.ClientRequest, _ strfmt.Registry) error {
				return nil
			}),
			Reader: runtime.ClientResponseReaderFunc(func(_ runtime.ClientResponse, _ runtime.Consumer) (interface{}, error) {
				return nil, nil
			}),
		})
		if assert.NoError(t, err) {
			assert.True(t, transport.overriden)
		}
	}
}

func TestRuntime_PreserveTrailingSlash(t *testing.T) {
	var redirected bool

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Add(runtime.HeaderContentType, runtime.JSONMime+";charset=utf-8")

		if req.URL.Path == "/api/tasks" {
			redirected = true
			return
		}
		if req.URL.Path == "/api/tasks/" {
			rw.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	hu, _ := url.Parse(server.URL)

	rt := New(hu.Host, "/", []string{"http"})

	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return nil
	})

	_, err := rt.Submit(&runtime.ClientOperation{
		ID:          "getTasks",
		Method:      "GET",
		PathPattern: "/api/tasks/",
		Params:      rwrtr,
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
			if redirected {
				return nil, errors.New("expected Submit to preserve trailing slashes - this caused a redirect")
			}
			if response.Code() == http.StatusOK {
				return nil, nil
			}
			return nil, errors.New("Generic error")
		}),
	})

	assert.NoError(t, err)
}

func TestRuntime_FallbackConsumer(t *testing.T) {
	result := `W3siY29tcGxldGVkIjpmYWxzZSwiY29udGVudCI6ImRHRnpheUF4SUdOdmJuUmxiblE9IiwiaWQiOjF9XQ==`
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Add(runtime.HeaderContentType, "application/x-task")
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte(result))
	}))
	defer server.Close()

	rwrtr := runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, _ strfmt.Registry) error {
		return req.SetBodyParam(bytes.NewBufferString("hello"))
	})

	hu, _ := url.Parse(server.URL)
	rt := New(hu.Host, "/", []string{"http"})

	// without the fallback consumer
	_, err := rt.Submit(&runtime.ClientOperation{
		ID:                 "getTasks",
		Method:             "POST",
		PathPattern:        "/",
		Schemes:            []string{"http"},
		ConsumesMediaTypes: []string{"application/octet-stream"},
		Params:             rwrtr,
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
			if response.Code() == 200 {
				var result []byte
				if err := consumer.Consume(response.Body(), &result); err != nil {
					return nil, err
				}
				return result, nil
			}
			return nil, errors.New("Generic error")
		}),
	})

	if assert.Error(t, err) {
		assert.Equal(t, `no consumer: "application/x-task"`, err.Error())
	}

	// add the fallback consumer
	rt.Consumers["*/*"] = rt.Consumers[runtime.DefaultMime]
	res, err := rt.Submit(&runtime.ClientOperation{
		ID:                 "getTasks",
		Method:             "POST",
		PathPattern:        "/",
		Schemes:            []string{"http"},
		ConsumesMediaTypes: []string{"application/octet-stream"},
		Params:             rwrtr,
		Reader: runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
			if response.Code() == 200 {
				var result []byte
				if err := consumer.Consume(response.Body(), &result); err != nil {
					return nil, err
				}
				return result, nil
			}
			return nil, errors.New("Generic error")
		}),
	})

	if assert.NoError(t, err) {
		assert.IsType(t, []byte{}, res)
		actual := res.([]byte)
		assert.EqualValues(t, result, actual)
	}
}
