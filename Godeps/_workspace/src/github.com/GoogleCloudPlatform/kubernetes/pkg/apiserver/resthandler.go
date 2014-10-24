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
	"path"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/httplog"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/golang/glog"
)

// CreateOrUpdate may be returned by a handler to distingush between a PUT
// that creates an object and a PUT that updates an object.
type CreateOrUpdate struct {
	Created bool
	Object  runtime.Object
}

func (*CreateOrUpdate) IsAnAPIObject() {}

type RESTHandler struct {
	storage         map[string]RESTStorage
	codec           runtime.Codec
	canonicalPrefix string
	selfLinker      runtime.SelfLinker
	ops             *Operations
	asyncOpWait     time.Duration
}

// ServeHTTP handles requests to all RESTStorage objects.
func (h *RESTHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	parts := splitPath(req.URL.Path)
	if len(parts) < 1 {
		notFound(w, req)
		return
	}
	storage := h.storage[parts[0]]
	if storage == nil {
		httplog.LogOf(req, w).Addf("'%v' has no storage object", parts[0])
		notFound(w, req)
		return
	}

	h.handleRESTStorage(parts, req, w, storage)
}

// Sets the SelfLink field of the object.
func (h *RESTHandler) setSelfLink(obj runtime.Object, req *http.Request) error {
	newURL := *req.URL
	newURL.Path = path.Join(h.canonicalPrefix, req.URL.Path)
	newURL.RawQuery = ""
	newURL.Fragment = ""
	return h.selfLinker.SetSelfLink(obj, newURL.String())
}

// Like setSelfLink, but appends the object's id.
func (h *RESTHandler) setSelfLinkAddID(obj runtime.Object, req *http.Request) error {
	id, err := h.selfLinker.ID(obj)
	if err != nil {
		return err
	}
	newURL := *req.URL
	newURL.Path = path.Join(h.canonicalPrefix, req.URL.Path, id)
	newURL.RawQuery = ""
	newURL.Fragment = ""
	return h.selfLinker.SetSelfLink(obj, newURL.String())
}

// curry adapts either of the self link setting functions into a function appropriate for operation's hook.
func curry(f func(runtime.Object, *http.Request) error, req *http.Request) func(runtime.Object) {
	return func(obj runtime.Object) {
		if err := f(obj, req); err != nil {
			glog.Errorf("unable to set self link for %#v: %v", obj, err)
		}
	}
}

// handleRESTStorage is the main dispatcher for a storage object.  It switches on the HTTP method, and then
// on path length, according to the following table:
//   Method     Path          Action
//   GET        /foo          list
//   GET        /foo/bar      get 'bar'
//   POST       /foo          create
//   PUT        /foo/bar      update 'bar'
//   DELETE     /foo/bar      delete 'bar'
// Returns 404 if the method/pattern doesn't match one of these entries
// The s accepts several query parameters:
//    sync=[false|true] Synchronous request (only applies to create, update, delete operations)
//    timeout=<duration> Timeout for synchronous requests, only applies if sync=true
//    labels=<label-selector> Used for filtering list operations
func (h *RESTHandler) handleRESTStorage(parts []string, req *http.Request, w http.ResponseWriter, storage RESTStorage) {
	// TODO for now, we perform all operations in the default namespace
	ctx := api.NewDefaultContext()
	sync := req.URL.Query().Get("sync") == "true"
	timeout := parseTimeout(req.URL.Query().Get("timeout"))
	switch req.Method {
	case "GET":
		switch len(parts) {
		case 1:
			label, err := labels.ParseSelector(req.URL.Query().Get("labels"))
			if err != nil {
				errorJSON(err, h.codec, w)
				return
			}
			field, err := labels.ParseSelector(req.URL.Query().Get("fields"))
			if err != nil {
				errorJSON(err, h.codec, w)
				return
			}
			list, err := storage.List(ctx, label, field)
			if err != nil {
				errorJSON(err, h.codec, w)
				return
			}
			if err := h.setSelfLink(list, req); err != nil {
				errorJSON(err, h.codec, w)
				return
			}
			writeJSON(http.StatusOK, h.codec, list, w)
		case 2:
			item, err := storage.Get(ctx, parts[1])
			if err != nil {
				errorJSON(err, h.codec, w)
				return
			}
			if err := h.setSelfLink(item, req); err != nil {
				errorJSON(err, h.codec, w)
				return
			}
			writeJSON(http.StatusOK, h.codec, item, w)
		default:
			notFound(w, req)
		}

	case "POST":
		if len(parts) != 1 {
			notFound(w, req)
			return
		}
		body, err := readBody(req)
		if err != nil {
			errorJSON(err, h.codec, w)
			return
		}
		obj := storage.New()
		err = h.codec.DecodeInto(body, obj)
		if err != nil {
			errorJSON(err, h.codec, w)
			return
		}
		out, err := storage.Create(ctx, obj)
		if err != nil {
			errorJSON(err, h.codec, w)
			return
		}
		op := h.createOperation(out, sync, timeout, curry(h.setSelfLinkAddID, req))
		h.finishReq(op, req, w)

	case "DELETE":
		if len(parts) != 2 {
			notFound(w, req)
			return
		}
		out, err := storage.Delete(ctx, parts[1])
		if err != nil {
			errorJSON(err, h.codec, w)
			return
		}
		op := h.createOperation(out, sync, timeout, nil)
		h.finishReq(op, req, w)

	case "PUT":
		if len(parts) != 2 {
			notFound(w, req)
			return
		}
		body, err := readBody(req)
		if err != nil {
			errorJSON(err, h.codec, w)
			return
		}
		obj := storage.New()
		err = h.codec.DecodeInto(body, obj)
		if err != nil {
			errorJSON(err, h.codec, w)
			return
		}
		out, err := storage.Update(ctx, obj)
		if err != nil {
			errorJSON(err, h.codec, w)
			return
		}
		op := h.createOperation(out, sync, timeout, curry(h.setSelfLink, req))
		h.finishReq(op, req, w)

	default:
		notFound(w, req)
	}
}

// createOperation creates an operation to process a channel response.
func (h *RESTHandler) createOperation(out <-chan runtime.Object, sync bool, timeout time.Duration, onReceive func(runtime.Object)) *Operation {
	op := h.ops.NewOperation(out, onReceive)
	if sync {
		op.WaitFor(timeout)
	} else if h.asyncOpWait != 0 {
		op.WaitFor(h.asyncOpWait)
	}
	return op
}

// finishReq finishes up a request, waiting until the operation finishes or, after a timeout, creating an
// Operation to receive the result and returning its ID down the writer.
func (h *RESTHandler) finishReq(op *Operation, req *http.Request, w http.ResponseWriter) {
	obj, complete := op.StatusOrResult()

	status := http.StatusOK
	switch stat := obj.(type) {
	case *api.Status:
		if stat.Code != 0 {
			status = stat.Code
		}
	case *CreateOrUpdate:
		obj = stat.Object
		if stat.Created {
			status = http.StatusCreated
		}
	}

	if complete {
		writeJSON(status, h.codec, obj, w)
	} else {
		writeJSON(http.StatusAccepted, h.codec, obj, w)
	}
}
