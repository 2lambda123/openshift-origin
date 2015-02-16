/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apiserver

import (
	"net/http"
	gpath "path"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/emicklei/go-restful"
)

// ContextFunc returns a Context given a request - a context must be returned
type ContextFunc func(req *restful.Request) api.Context

// ScopeNamer handles accessing names from requests and objects
type ScopeNamer interface {
	// Namespace returns the appropriate namespace value from the request (may be empty) or an
	// error.
	Namespace(req *restful.Request) (namespace string, err error)
	// Name returns the name from the request, and an optional namespace value if this is a namespace
	// scoped call. An error is returned if the name is not available.
	Name(req *restful.Request) (namespace, name string, err error)
	// ObjectName returns the namespace and name from an object if they exist, or an error if the object
	// does not support names.
	ObjectName(obj runtime.Object) (namespace, name string, err error)
	// SetSelfLink sets the provided URL onto the object. The method should return nil if the object
	// does not support selfLinks.
	SetSelfLink(obj runtime.Object, url string) error
	// GenerateLink creates a path and query for a given runtime object that represents the canonical path.
	GenerateLink(req *restful.Request, obj runtime.Object) (path, query string, err error)
}

// GetResource returns a function that handles retrieving a single resource from a RESTStorage object.
func GetResource(r RESTGetter, ctxFn ContextFunc, namer ScopeNamer, codec runtime.Codec) restful.RouteFunction {
	return func(req *restful.Request, res *restful.Response) {
		w := res.ResponseWriter
		namespace, name, err := namer.Name(req)
		if err != nil {
			notFound(w, req.Request)
			return
		}
		ctx := ctxFn(req)
		ctx = api.WithNamespace(ctx, namespace)

		result, err := r.Get(ctx, name)
		if err != nil {
			errorJSON(err, codec, w)
			return
		}
		if err := setSelfLink(result, req, namer); err != nil {
			errorJSON(err, codec, w)
			return
		}
		writeJSON(http.StatusOK, codec, result, w)
	}
}

// ListResource returns a function that handles retrieving a list of resources from a RESTStorage object.
func ListResource(r RESTLister, ctxFn ContextFunc, namer ScopeNamer, codec runtime.Codec) restful.RouteFunction {
	return func(req *restful.Request, res *restful.Response) {
		w := res.ResponseWriter

		namespace, err := namer.Namespace(req)
		if err != nil {
			notFound(w, req.Request)
			return
		}
		ctx := ctxFn(req)
		ctx = api.WithNamespace(ctx, namespace)

		label, err := labels.ParseSelector(req.Request.URL.Query().Get("labels"))
		if err != nil {
			errorJSON(err, codec, w)
			return
		}
		field, err := labels.ParseSelector(req.Request.URL.Query().Get("fields"))
		if err != nil {
			errorJSON(err, codec, w)
			return
		}

		result, err := r.List(ctx, label, field)
		if err != nil {
			errorJSON(err, codec, w)
			return
		}
		if err := setSelfLink(result, req, namer); err != nil {
			errorJSON(err, codec, w)
			return
		}
		writeJSON(http.StatusOK, codec, result, w)
	}
}

// CreateResource returns a function that will handle a resource creation.
func CreateResource(r RESTCreater, ctxFn ContextFunc, namer ScopeNamer, codec runtime.Codec, resource string, admit admission.Interface) restful.RouteFunction {
	return func(req *restful.Request, res *restful.Response) {
		w := res.ResponseWriter

		// TODO: we either want to remove timeout or document it (if we document, move timeout out of this function and declare it in api_installer)
		timeout := parseTimeout(req.Request.URL.Query().Get("timeout"))

		namespace, err := namer.Namespace(req)
		if err != nil {
			notFound(w, req.Request)
			return
		}
		ctx := ctxFn(req)
		ctx = api.WithNamespace(ctx, namespace)

		body, err := readBody(req.Request)
		if err != nil {
			errorJSON(err, codec, w)
			return
		}

		obj := r.New()
		if err := codec.DecodeInto(body, obj); err != nil {
			errorJSON(err, codec, w)
			return
		}

		err = admit.Admit(admission.NewAttributesRecord(obj, namespace, resource, "CREATE"))
		if err != nil {
			errorJSON(err, codec, w)
			return
		}

		result, err := finishRequest(timeout, func() (runtime.Object, error) {
			out, err := r.Create(ctx, obj)
			if status, ok := out.(*api.Status); ok && err == nil && status.Code == 0 {
				status.Code = http.StatusCreated
			}
			return out, err
		})
		if err != nil {
			errorJSON(err, codec, w)
			return
		}

		if err := setSelfLink(result, req, namer); err != nil {
			errorJSON(err, codec, w)
			return
		}

		writeJSON(http.StatusCreated, codec, result, w)
	}
}

// UpdateResource returns a function that will handle a resource update
func UpdateResource(r RESTUpdater, ctxFn ContextFunc, namer ScopeNamer, codec runtime.Codec, resource string, admit admission.Interface) restful.RouteFunction {
	return func(req *restful.Request, res *restful.Response) {
		w := res.ResponseWriter

		// TODO: we either want to remove timeout or document it (if we document, move timeout out of this function and declare it in api_installer)
		timeout := parseTimeout(req.Request.URL.Query().Get("timeout"))

		namespace, name, err := namer.Name(req)
		if err != nil {
			notFound(w, req.Request)
			return
		}
		ctx := ctxFn(req)
		ctx = api.WithNamespace(ctx, namespace)

		body, err := readBody(req.Request)
		if err != nil {
			errorJSON(err, codec, w)
			return
		}

		obj := r.New()
		if err := codec.DecodeInto(body, obj); err != nil {
			errorJSON(err, codec, w)
			return
		}

		// check the provided name against the request
		if objNamespace, objName, err := namer.ObjectName(obj); err == nil {
			if objName != name {
				errorJSON(errors.NewBadRequest("the name of the object does not match the name on the URL"), codec, w)
				return
			}
			if len(namespace) > 0 {
				if len(objNamespace) > 0 && objNamespace != namespace {
					errorJSON(errors.NewBadRequest("the namespace of the object does not match the namespace on the request"), codec, w)
					return
				}
			}
		}

		err = admit.Admit(admission.NewAttributesRecord(obj, namespace, resource, "UPDATE"))
		if err != nil {
			errorJSON(err, codec, w)
			return
		}

		wasCreated := false
		result, err := finishRequest(timeout, func() (runtime.Object, error) {
			obj, created, err := r.Update(ctx, obj)
			wasCreated = created
			return obj, err
		})
		if err != nil {
			errorJSON(err, codec, w)
			return
		}

		if err := setSelfLink(result, req, namer); err != nil {
			errorJSON(err, codec, w)
			return
		}

		status := http.StatusOK
		if wasCreated {
			status = http.StatusCreated
		}
		writeJSON(status, codec, result, w)
	}
}

// DeleteResource returns a function that will handle a resource deletion
func DeleteResource(r RESTDeleter, ctxFn ContextFunc, namer ScopeNamer, codec runtime.Codec, resource, kind string, admit admission.Interface) restful.RouteFunction {
	return func(req *restful.Request, res *restful.Response) {
		w := res.ResponseWriter

		// TODO: we either want to remove timeout or document it (if we document, move timeout out of this function and declare it in api_installer)
		timeout := parseTimeout(req.Request.URL.Query().Get("timeout"))

		namespace, name, err := namer.Name(req)
		if err != nil {
			notFound(w, req.Request)
			return
		}
		ctx := ctxFn(req)
		if len(namespace) > 0 {
			ctx = api.WithNamespace(ctx, namespace)
		}

		err = admit.Admit(admission.NewAttributesRecord(nil, namespace, resource, "DELETE"))
		if err != nil {
			errorJSON(err, codec, w)
			return
		}

		result, err := finishRequest(timeout, func() (runtime.Object, error) {
			return r.Delete(ctx, name)
		})
		if err != nil {
			errorJSON(err, codec, w)
			return
		}

		// if the RESTDeleter returns a nil object, fill out a status. Callers may return a valid
		// object with the response.
		if result == nil {
			result = &api.Status{
				Status: api.StatusSuccess,
				Code:   http.StatusOK,
				Details: &api.StatusDetails{
					ID:   name,
					Kind: kind,
				},
			}
		} else {
			// when a non-status response is returned, set the self link
			if _, ok := result.(*api.Status); !ok {
				if err := setSelfLink(result, req, namer); err != nil {
					errorJSON(err, codec, w)
					return
				}
			}
		}
		writeJSON(http.StatusOK, codec, result, w)
	}
}

// resultFunc is a function that returns a rest result and can be run in a goroutine
type resultFunc func() (runtime.Object, error)

// finishRequest makes a given resultFunc asynchronous and handles errors returned by the response.
// Any api.Status object returned is considered an "error", which interrupts the normal response flow.
func finishRequest(timeout time.Duration, fn resultFunc) (result runtime.Object, err error) {
	ch := make(chan runtime.Object)
	errCh := make(chan error)
	go func() {
		if result, err := fn(); err != nil {
			errCh <- err
		} else {
			ch <- result
		}
	}()

	select {
	case result = <-ch:
		if status, ok := result.(*api.Status); ok {
			return nil, errors.FromObject(status)
		}
		return result, nil
	case err = <-errCh:
		return nil, err
	case <-time.After(timeout):
		return nil, errors.NewTimeoutError("request did not complete within allowed duration")
	}
}

// setSelfLink sets the self link of an object (or the child items in a list) to the base URL of the request
// plus the path and query generated by the provided linkFunc
func setSelfLink(obj runtime.Object, req *restful.Request, namer ScopeNamer) error {
	if runtime.IsListType(obj) {
		// Set self-link of objects in the list.
		items, err := runtime.ExtractList(obj)
		if err != nil {
			return err
		}
		for i := range items {
			if err := setSelfLink(items[i], req, namer); err != nil {
				return err
			}
		}
		return runtime.SetList(obj, items)
	}

	path, query, err := namer.GenerateLink(req, obj)
	if err == errEmptyName {
		return nil
	}
	if err != nil {
		return err
	}

	newURL := *req.Request.URL
	// use only canonical paths
	newURL.Path = gpath.Clean(path)
	newURL.RawQuery = query
	newURL.Fragment = ""

	return namer.SetSelfLink(obj, newURL.String())
}
