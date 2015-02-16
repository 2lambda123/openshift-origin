/*
Copyright 2015 Google Inc. All rights reserved.

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
	"fmt"
	"net/http"
	"net/url"
	gpath "path"
	"reflect"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/emicklei/go-restful"
)

type APIInstaller struct {
	group   *APIGroupVersion
	prefix  string // Path prefix where API resources are to be registered.
	version string // The API version being installed.
}

// Struct capturing information about an action ("GET", "POST", "WATCH", PROXY", etc).
type action struct {
	Verb   string               // Verb identifying the action ("GET", "POST", "WATCH", PROXY", etc).
	Path   string               // The path of the action
	Params []*restful.Parameter // List of parameters associated with the action.
	Namer  ScopeNamer
}

// errEmptyName is returned when API requests do not fill the name section of the path.
var errEmptyName = fmt.Errorf("name must be provided")

// Installs handlers for API resources.
func (a *APIInstaller) Install() (ws *restful.WebService, errors []error) {
	errors = make([]error, 0)

	// Create the WebService.
	ws = a.newWebService()

	// Initialize the custom handlers.
	watchHandler := (&WatchHandler{
		storage: a.group.storage,
		codec:   a.group.codec,
		prefix:  a.group.prefix,
		linker:  a.group.linker,
		info:    a.group.info,
	})
	redirectHandler := (&RedirectHandler{a.group.storage, a.group.codec, a.group.context, a.group.info})
	proxyHandler := (&ProxyHandler{a.prefix + "/proxy/", a.group.storage, a.group.codec, a.group.context, a.group.info})

	for path, storage := range a.group.storage {
		if err := a.registerResourceHandlers(path, storage, ws, watchHandler, redirectHandler, proxyHandler); err != nil {
			errors = append(errors, err)
		}
	}
	return ws, errors
}

func (a *APIInstaller) newWebService() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path(a.prefix)
	ws.Doc("API at " + a.prefix + " version " + a.version)
	// TODO: change to restful.MIME_JSON when we set content type in client
	ws.Consumes("*/*")
	ws.Produces(restful.MIME_JSON)
	ws.ApiVersion(a.version)
	return ws
}

func (a *APIInstaller) registerResourceHandlers(path string, storage RESTStorage, ws *restful.WebService, watchHandler http.Handler, redirectHandler http.Handler, proxyHandler http.Handler) error {
	codec := a.group.codec
	admit := a.group.admit
	context := a.group.context
	resource := path

	object := storage.New()
	// TODO: add scheme to APIInstaller rather than using api.Scheme
	_, kind, err := api.Scheme.ObjectVersionAndKind(object)
	if err != nil {
		return err
	}
	versionedPtr, err := api.Scheme.New(a.version, kind)
	if err != nil {
		return err
	}
	versionedObject := indirectArbitraryPointer(versionedPtr)

	var versionedList interface{}
	if lister, ok := storage.(RESTLister); ok {
		list := lister.NewList()
		_, listKind, err := api.Scheme.ObjectVersionAndKind(list)
		versionedListPtr, err := api.Scheme.New(a.version, listKind)
		if err != nil {
			return err
		}
		versionedList = indirectArbitraryPointer(versionedListPtr)
	}

	mapping, err := a.group.mapper.RESTMapping(kind, a.version)
	if err != nil {
		return err
	}

	// what verbs are supported by the storage, used to know what verbs we support per path
	storageVerbs := map[string]bool{}
	creater, ok := storage.(RESTCreater)
	if ok {
		storageVerbs["RESTCreater"] = true
	}
	lister, ok := storage.(RESTLister)
	if ok {
		storageVerbs["RESTLister"] = true
	}
	getter, ok := storage.(RESTGetter)
	if ok {
		storageVerbs["RESTGetter"] = true
	}
	deleter, ok := storage.(RESTDeleter)
	if ok {
		storageVerbs["RESTDeleter"] = true
	}
	updater, ok := storage.(RESTUpdater)
	if ok {
		storageVerbs["RESTUpdater"] = true
	}
	if _, ok := storage.(ResourceWatcher); ok {
		storageVerbs["ResourceWatcher"] = true
	}
	if _, ok := storage.(Redirector); ok {
		storageVerbs["Redirector"] = true
	}

	var ctxFn ContextFunc
	ctxFn = func(req *restful.Request) api.Context {
		if ctx, ok := context.Get(req.Request); ok {
			return ctx
		}
		return api.NewContext()
	}

	allowWatchList := storageVerbs["ResourceWatcher"] && storageVerbs["RESTLister"] // watching on lists is allowed only for kinds that support both watch and list.
	scope := mapping.Scope
	nameParam := ws.PathParameter("name", "name of the "+kind).DataType("string")
	params := []*restful.Parameter{}
	actions := []action{}

	// Get the list of actions for the given scope.
	if scope.Name() != meta.RESTScopeNameNamespace {
		itemPath := path + "/{name}"
		nameParams := append(params, nameParam)
		namer := rootScopeNaming{scope, a.group.linker, gpath.Join(a.prefix, itemPath)}

		// Handler for standard REST verbs (GET, PUT, POST and DELETE).
		actions = appendIf(actions, action{"LIST", path, params, namer}, storageVerbs["RESTLister"])
		actions = appendIf(actions, action{"POST", path, params, namer}, storageVerbs["RESTCreater"])
		actions = appendIf(actions, action{"WATCHLIST", "/watch/" + path, params, namer}, allowWatchList)

		actions = appendIf(actions, action{"GET", itemPath, nameParams, namer}, storageVerbs["RESTGetter"])
		actions = appendIf(actions, action{"PUT", itemPath, nameParams, namer}, storageVerbs["RESTUpdater"])
		actions = appendIf(actions, action{"DELETE", itemPath, nameParams, namer}, storageVerbs["RESTDeleter"])
		actions = appendIf(actions, action{"WATCH", "/watch/" + itemPath, nameParams, namer}, storageVerbs["ResourceWatcher"])
		actions = appendIf(actions, action{"REDIRECT", "/redirect/" + itemPath, nameParams, namer}, storageVerbs["Redirector"])
		actions = appendIf(actions, action{"PROXY", "/proxy/" + itemPath + "/{path:*}", nameParams, namer}, storageVerbs["Redirector"])
		actions = appendIf(actions, action{"PROXY", "/proxy/" + itemPath, nameParams, namer}, storageVerbs["Redirector"])

	} else {
		// v1beta3 format with namespace in path
		if scope.ParamPath() {
			// Handler for standard REST verbs (GET, PUT, POST and DELETE).
			namespaceParam := ws.PathParameter(scope.ParamName(), scope.ParamDescription()).DataType("string")
			namespacedPath := scope.ParamName() + "/{" + scope.ParamName() + "}/" + path
			namespaceParams := []*restful.Parameter{namespaceParam}

			itemPath := namespacedPath + "/{name}"
			nameParams := append(namespaceParams, nameParam)
			namer := scopeNaming{scope, a.group.linker, gpath.Join(a.prefix, itemPath), false}

			actions = appendIf(actions, action{"LIST", namespacedPath, namespaceParams, namer}, storageVerbs["RESTLister"])
			actions = appendIf(actions, action{"POST", namespacedPath, namespaceParams, namer}, storageVerbs["RESTCreater"])
			actions = appendIf(actions, action{"WATCHLIST", "/watch/" + namespacedPath, namespaceParams, namer}, allowWatchList)

			actions = appendIf(actions, action{"GET", itemPath, nameParams, namer}, storageVerbs["RESTGetter"])
			actions = appendIf(actions, action{"PUT", itemPath, nameParams, namer}, storageVerbs["RESTUpdater"])
			actions = appendIf(actions, action{"DELETE", itemPath, nameParams, namer}, storageVerbs["RESTDeleter"])
			actions = appendIf(actions, action{"WATCH", "/watch/" + itemPath, nameParams, namer}, storageVerbs["ResourceWatcher"])
			actions = appendIf(actions, action{"REDIRECT", "/redirect/" + itemPath, nameParams, namer}, storageVerbs["Redirector"])
			actions = appendIf(actions, action{"PROXY", "/proxy/" + itemPath + "/{path:*}", nameParams, namer}, storageVerbs["Redirector"])
			actions = appendIf(actions, action{"PROXY", "/proxy/" + itemPath, nameParams, namer}, storageVerbs["Redirector"])

			// list across namespace.
			namer = scopeNaming{scope, a.group.linker, gpath.Join(a.prefix, itemPath), true}
			actions = appendIf(actions, action{"LIST", path, params, namer}, storageVerbs["RESTLister"])
			actions = appendIf(actions, action{"WATCHLIST", "/watch/" + path, params, namer}, allowWatchList)

		} else {
			// Handler for standard REST verbs (GET, PUT, POST and DELETE).
			// v1beta1/v1beta2 format where namespace was a query parameter
			namespaceParam := ws.QueryParameter(scope.ParamName(), scope.ParamDescription()).DataType("string")
			namespaceParams := []*restful.Parameter{namespaceParam}

			itemPath := path + "/{name}"
			nameParams := append(namespaceParams, nameParam)
			namer := legacyScopeNaming{scope, a.group.linker, gpath.Join(a.prefix, itemPath)}

			actions = appendIf(actions, action{"LIST", path, namespaceParams, namer}, storageVerbs["RESTLister"])
			actions = appendIf(actions, action{"POST", path, namespaceParams, namer}, storageVerbs["RESTCreater"])
			actions = appendIf(actions, action{"WATCHLIST", "/watch/" + path, namespaceParams, namer}, allowWatchList)

			actions = appendIf(actions, action{"GET", itemPath, nameParams, namer}, storageVerbs["RESTGetter"])
			actions = appendIf(actions, action{"PUT", itemPath, nameParams, namer}, storageVerbs["RESTUpdater"])
			actions = appendIf(actions, action{"DELETE", itemPath, nameParams, namer}, storageVerbs["RESTDeleter"])
			actions = appendIf(actions, action{"WATCH", "/watch/" + itemPath, nameParams, namer}, storageVerbs["ResourceWatcher"])
			actions = appendIf(actions, action{"REDIRECT", "/redirect/" + itemPath, nameParams, namer}, storageVerbs["Redirector"])
			actions = appendIf(actions, action{"PROXY", "/proxy/" + itemPath + "/{path:*}", nameParams, namer}, storageVerbs["Redirector"])
			actions = appendIf(actions, action{"PROXY", "/proxy/" + itemPath, nameParams, namer}, storageVerbs["Redirector"])
		}
	}

	// Create Routes for the actions.
	// TODO: Add status documentation using Returns()
	// Errors (see api/errors/errors.go as well as go-restful router):
	// http.StatusNotFound, http.StatusMethodNotAllowed,
	// http.StatusUnsupportedMediaType, http.StatusNotAcceptable,
	// http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
	// http.StatusRequestTimeout, http.StatusConflict, http.StatusPreconditionFailed,
	// 422 (StatusUnprocessableEntity), http.StatusInternalServerError,
	// http.StatusServiceUnavailable
	// and api error codes
	// Note that if we specify a versioned Status object here, we may need to
	// create one for the tests, also
	// Success:
	// http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent
	//
	// test/integration/auth_test.go is currently the most comprehensive status code test

	for _, action := range actions {
		m := monitorFilter(action.Verb, resource)
		switch action.Verb {
		case "GET": // Get a resource.
			route := ws.GET(action.Path).To(GetResource(getter, ctxFn, action.Namer, codec)).
				Filter(m).
				Doc("read the specified " + kind).
				Operation("read" + kind).
				Writes(versionedObject)
			addParams(route, action.Params)
			ws.Route(route)
		case "LIST": // List all resources of a kind.
			route := ws.GET(action.Path).To(ListResource(lister, ctxFn, action.Namer, codec)).
				Filter(m).
				Doc("list objects of kind " + kind).
				Operation("list" + kind).
				Writes(versionedList)
			addParams(route, action.Params)
			ws.Route(route)
		case "PUT": // Update a resource.
			route := ws.PUT(action.Path).To(UpdateResource(updater, ctxFn, action.Namer, codec, resource, admit)).
				Filter(m).
				Doc("update the specified " + kind).
				Operation("update" + kind).
				Reads(versionedObject)
			addParams(route, action.Params)
			ws.Route(route)
		case "POST": // Create a resource.
			route := ws.POST(action.Path).To(CreateResource(creater, ctxFn, action.Namer, codec, resource, admit)).
				Filter(m).
				Doc("create a " + kind).
				Operation("create" + kind).
				Reads(versionedObject)
			addParams(route, action.Params)
			ws.Route(route)
		case "DELETE": // Delete a resource.
			route := ws.DELETE(action.Path).To(DeleteResource(deleter, ctxFn, action.Namer, codec, resource, kind, admit)).
				Filter(m).
				Doc("delete a " + kind).
				Operation("delete" + kind)
			addParams(route, action.Params)
			ws.Route(route)
		case "WATCH": // Watch a resource.
			route := ws.GET(action.Path).To(restfulStripPrefix(a.prefix+"/watch", watchHandler)).
				Filter(m).
				Doc("watch a particular " + kind).
				Operation("watch" + kind).
				Writes(versionedObject)
			addParams(route, action.Params)
			ws.Route(route)
		case "WATCHLIST": // Watch all resources of a kind.
			route := ws.GET(action.Path).To(restfulStripPrefix(a.prefix+"/watch", watchHandler)).
				Filter(m).
				Doc("watch a list of " + kind).
				Operation("watch" + kind + "list").
				Writes(versionedList)
			addParams(route, action.Params)
			ws.Route(route)
		case "REDIRECT": // Get the redirect URL for a resource.
			route := ws.GET(action.Path).To(restfulStripPrefix(a.prefix+"/redirect", redirectHandler)).
				Filter(m).
				Doc("redirect GET request to " + kind).
				Operation("redirect" + kind).
				Produces("*/*").
				Consumes("*/*")
			addParams(route, action.Params)
			ws.Route(route)
		case "PROXY": // Proxy requests to a resource.
			// Accept all methods as per https://github.com/GoogleCloudPlatform/kubernetes/issues/3996
			addProxyRoute(ws, "GET", a.prefix, action.Path, proxyHandler, kind, resource, action.Params)
			addProxyRoute(ws, "PUT", a.prefix, action.Path, proxyHandler, kind, resource, action.Params)
			addProxyRoute(ws, "POST", a.prefix, action.Path, proxyHandler, kind, resource, action.Params)
			addProxyRoute(ws, "DELETE", a.prefix, action.Path, proxyHandler, kind, resource, action.Params)
		default:
			return fmt.Errorf("unrecognized action verb: %s", action.Verb)
		}
		// Note: update GetAttribs() when adding a custom handler.
	}
	return nil
}

// rootScopeNaming reads only names from a request and ignores namespaces. It implements ScopeNamer
// for root scoped resources.
type rootScopeNaming struct {
	scope meta.RESTScope
	runtime.SelfLinker
	itemPath string
}

// rootScopeNaming implements ScopeNamer
var _ ScopeNamer = rootScopeNaming{}

// Namespace returns an empty string because root scoped objects have no namespace.
func (n rootScopeNaming) Namespace(req *restful.Request) (namespace string, err error) {
	return "", nil
}

// Name returns the name from the path and an empty string for namespace, or an error if the
// name is empty.
func (n rootScopeNaming) Name(req *restful.Request) (namespace, name string, err error) {
	name = req.PathParameter("name")
	if len(name) == 0 {
		return "", "", errEmptyName
	}
	return "", name, nil
}

// GenerateLink returns the appropriate path and query to locate an object by its canonical path.
func (n rootScopeNaming) GenerateLink(req *restful.Request, obj runtime.Object) (path, query string, err error) {
	_, name, err := n.ObjectName(obj)
	if err != nil {
		return "", "", err
	}
	if len(name) == 0 {
		_, name, err = n.Name(req)
		if err != nil {
			return "", "", err
		}
	}
	path = strings.Replace(n.itemPath, "{name}", name, 1)
	return path, "", nil
}

// ObjectName returns the name set on the object, or an error if the
// name cannot be returned. Namespace is empty
// TODO: distinguish between objects with name/namespace and without via a specific error.
func (n rootScopeNaming) ObjectName(obj runtime.Object) (namespace, name string, err error) {
	name, err = n.SelfLinker.Name(obj)
	if err != nil {
		return "", "", err
	}
	if len(name) == 0 {
		return "", "", errEmptyName
	}
	return "", name, nil
}

// scopeNaming returns naming information from a request. It implements ScopeNamer for
// namespace scoped resources.
type scopeNaming struct {
	scope meta.RESTScope
	runtime.SelfLinker
	itemPath      string
	allNamespaces bool
}

// scopeNaming implements ScopeNamer
var _ ScopeNamer = scopeNaming{}

// Namespace returns the namespace from the path or the default.
func (n scopeNaming) Namespace(req *restful.Request) (namespace string, err error) {
	if n.allNamespaces {
		return "", nil
	}
	namespace = req.PathParameter(n.scope.ParamName())
	if len(namespace) == 0 {
		// programmer error
		return "", fmt.Errorf("no namespace parameter found on request")
	}
	return namespace, nil
}

// Name returns the name from the path, the namespace (or default), or an error if the
// name is empty.
func (n scopeNaming) Name(req *restful.Request) (namespace, name string, err error) {
	namespace, _ = n.Namespace(req)
	name = req.PathParameter("name")
	if len(name) == 0 {
		return "", "", errEmptyName
	}
	return
}

// GenerateLink returns the appropriate path and query to locate an object by its canonical path.
func (n scopeNaming) GenerateLink(req *restful.Request, obj runtime.Object) (path, query string, err error) {
	namespace, name, err := n.ObjectName(obj)
	if err != nil {
		return "", "", err
	}
	if len(namespace) == 0 && len(name) == 0 {
		namespace, name, err = n.Name(req)
		if err != nil {
			return "", "", err
		}
	}
	path = strings.Replace(n.itemPath, "{name}", name, 1)
	path = strings.Replace(path, "{"+n.scope.ParamName()+"}", namespace, 1)
	return path, "", nil
}

// ObjectName returns the name and namespace set on the object, or an error if the
// name cannot be returned.
// TODO: distinguish between objects with name/namespace and without via a specific error.
func (n scopeNaming) ObjectName(obj runtime.Object) (namespace, name string, err error) {
	name, err = n.SelfLinker.Name(obj)
	if err != nil {
		return "", "", err
	}
	namespace, err = n.SelfLinker.Namespace(obj)
	if err != nil {
		return "", "", err
	}
	return namespace, name, err
}

// legacyScopeNaming modifies a scopeNaming to read namespace from the query. It implements
// ScopeNamer for older query based namespace parameters.
type legacyScopeNaming struct {
	scope meta.RESTScope
	runtime.SelfLinker
	itemPath string
}

// legacyScopeNaming implements ScopeNamer
var _ ScopeNamer = legacyScopeNaming{}

// Namespace returns the namespace from the query or the default.
func (n legacyScopeNaming) Namespace(req *restful.Request) (namespace string, err error) {
	values, ok := req.Request.URL.Query()[n.scope.ParamName()]
	if !ok || len(values) == 0 {
		return api.NamespaceDefault, nil
	}
	return values[0], nil
}

// Name returns the name from the path, the namespace (or default), or an error if the
// name is empty.
func (n legacyScopeNaming) Name(req *restful.Request) (namespace, name string, err error) {
	namespace, _ = n.Namespace(req)
	name = req.PathParameter("name")
	if len(name) == 0 {
		return "", "", errEmptyName
	}
	return namespace, name, nil
}

// GenerateLink returns the appropriate path and query to locate an object by its canonical path.
func (n legacyScopeNaming) GenerateLink(req *restful.Request, obj runtime.Object) (path, query string, err error) {
	namespace, name, err := n.ObjectName(obj)
	if err != nil {
		return "", "", err
	}
	if len(name) == 0 {
		return "", "", errEmptyName
	}
	path = strings.Replace(n.itemPath, "{name}", name, -1)
	values := make(url.Values)
	values.Set(n.scope.ParamName(), namespace)
	query = values.Encode()
	return path, query, nil
}

// ObjectName returns the name and namespace set on the object, or an error if the
// name cannot be returned.
// TODO: distinguish between objects with name/namespace and without via a specific error.
func (n legacyScopeNaming) ObjectName(obj runtime.Object) (namespace, name string, err error) {
	name, err = n.SelfLinker.Name(obj)
	if err != nil {
		return "", "", err
	}
	namespace, err = n.SelfLinker.Namespace(obj)
	if err != nil {
		return "", "", err
	}
	return namespace, name, err
}

// This magic incantation returns *ptrToObject for an arbitrary pointer
func indirectArbitraryPointer(ptrToObject interface{}) interface{} {
	return reflect.Indirect(reflect.ValueOf(ptrToObject)).Interface()
}

func appendIf(actions []action, a action, shouldAppend bool) []action {
	if shouldAppend {
		actions = append(actions, a)
	}
	return actions
}

// Returns a restful RouteFunction that calls the given handler after stripping prefix from the request path.
func restfulStripPrefix(prefix string, handler http.Handler) restful.RouteFunction {
	return func(restReq *restful.Request, restResp *restful.Response) {
		http.StripPrefix(prefix, handler).ServeHTTP(restResp.ResponseWriter, restReq.Request)
	}
}

func addProxyRoute(ws *restful.WebService, method string, prefix string, path string, proxyHandler http.Handler, kind, resource string, params []*restful.Parameter) {
	proxyRoute := ws.Method(method).Path(path).To(restfulStripPrefix(prefix+"/proxy", proxyHandler)).
		Filter(monitorFilter("PROXY", resource)).
		Doc("proxy " + method + " requests to " + kind).
		Operation("proxy" + method + kind).
		Produces("*/*").
		Consumes("*/*")
	addParams(proxyRoute, params)
	ws.Route(proxyRoute)
}

func addParams(route *restful.RouteBuilder, params []*restful.Parameter) {
	for _, param := range params {
		route.Param(param)
	}
}
