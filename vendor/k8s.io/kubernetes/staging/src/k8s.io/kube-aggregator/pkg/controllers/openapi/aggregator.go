/*
Copyright 2017 The Kubernetes Authors.

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

package openapi

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	restful "github.com/emicklei/go-restful"
	"github.com/go-openapi/spec"
	"k8s.io/apiserver/pkg/server/mux"

	"k8s.io/apiserver/pkg/server"
	"k8s.io/kube-aggregator/pkg/apis/apiregistration"
	"k8s.io/kube-openapi/pkg/aggregator"
	"k8s.io/kube-openapi/pkg/builder"
	"k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/handler"
)

const (
	aggregatorUser                = "system:aggregator"
	specDownloadTimeout           = 60 * time.Second
	localDelegateChainNamePrefix  = "k8s_internal_local_delegation_chain_"
	localDelegateChainNamePattern = localDelegateChainNamePrefix + "%010d"

	// A randomly generated UUID to differentiate local and remote eTags.
	locallyGeneratedEtagPrefix = "\"6E8F849B434D4B98A569B9D7718876E9-"
)

type specAggregator struct {
	// mutex protects all members of this struct.
	rwMutex sync.RWMutex

	// Map of API Services' OpenAPI specs by their name
	openAPISpecs map[string]*openAPISpecInfo

	// provided for dynamic OpenAPI spec
	openAPIVersionedService *handler.OpenAPIService
}

var _ AggregationManager = &specAggregator{}

// IsLocalAPIService returns true for local specs from delegates.
func IsLocalAPIService(apiServiceName string) bool {
	return strings.HasPrefix(apiServiceName, localDelegateChainNamePrefix)
}

// This function is not thread safe as it only being called on startup.
func (s *specAggregator) addLocalSpec(spec *spec.Swagger, localHandler http.Handler, name, etag string) {
	localAPIService := apiregistration.APIService{}
	localAPIService.Name = name
	s.openAPISpecs[name] = &openAPISpecInfo{
		etag:       etag,
		apiService: localAPIService,
		handler:    localHandler,
		spec:       spec,
		local:      true,
	}
}

// GetAPIServicesName returns the names of APIServices recorded in specAggregator.openAPISpecs.
// We use this function to pass the names of local APIServices to the controller in this package,
// so that the controller can periodically sync the OpenAPI spec from delegation API servers.
func (s *specAggregator) GetAPIServiceNames() []string {
	names := make([]string, len(s.openAPISpecs))
	for key := range s.openAPISpecs {
		names = append(names, key)
	}
	return names
}

// BuildAndRegisterAggregator registered OpenAPI aggregator handler. This function is not thread safe as it only being called on startup.
func BuildAndRegisterAggregator(downloader *Downloader, delegationTarget server.DelegationTarget, webServices []*restful.WebService,
	config *common.Config, pathHandler *mux.PathRecorderMux) (AggregationManager, error) {
	s := &specAggregator{
		openAPISpecs: map[string]*openAPISpecInfo{},
	}

	i := 0
	// Build Aggregator's spec
	aggregatorOpenAPISpec, err := builder.BuildOpenAPISpec(
		webServices, config)
	if err != nil {
		return nil, err
	}

	// Reserving non-name spec for aggregator's Spec.
	s.addLocalSpec(aggregatorOpenAPISpec, nil, fmt.Sprintf(localDelegateChainNamePattern, i), "")
	i++
	for delegate := delegationTarget; delegate != nil; delegate = delegate.NextDelegate() {
		handler := delegate.UnprotectedHandler()
		if handler == nil {
			continue
		}
		delegateSpec, etag, _, err := downloader.Download(handler, "")
		if err != nil {
			return nil, err
		}
		if delegateSpec == nil {
			continue
		}
		s.addLocalSpec(delegateSpec, handler, fmt.Sprintf(localDelegateChainNamePattern, i), etag)
		i++
	}

	// Build initial spec to serve.
	specToServe, err := s.buildOpenAPISpec()
	if err != nil {
		return nil, err
	}

	s.openAPIVersionedService, err = handler.RegisterOpenAPIVersionedService(
		specToServe, "/openapi/v2", pathHandler)
	if err != nil {
		return nil, err
	}

	// NOTE: [DEPRECATION] We will announce deprecation for format-separated endpoints for OpenAPI spec,
	// and switch to a single /openapi/v2 endpoint in Kubernetes 1.10. The design doc and deprecation process
	// are tracked at: https://docs.google.com/document/d/19lEqE9lc4yHJ3WJAJxS_G7TcORIJXGHyq3wpwcH28nU.
	pathHandler.Handle("/swagger.json", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// forward request to /openapi/v2
		clone := *req
		u := *req.URL
		u.Path = "/openapi/v2"
		u.RawPath = "/openapi/v2"
		clone.URL = &u
		pathHandler.ServeHTTP(w, &clone)
	}))

	return s, nil
}

// openAPISpecInfo is used to store OpenAPI spec with its priority.
// It can be used to sort specs with their priorities.
type openAPISpecInfo struct {
	apiService apiregistration.APIService

	// Specification of this API Service. If null then the spec is not loaded yet.
	spec    *spec.Swagger
	handler http.Handler
	etag    string
	local   bool
}

// byPriority can be used in sort.Sort to sort specs with their priorities.
type byPriority struct {
	specs           []openAPISpecInfo
	groupPriorities map[string]int32
}

func (a byPriority) Len() int      { return len(a.specs) }
func (a byPriority) Swap(i, j int) { a.specs[i], a.specs[j] = a.specs[j], a.specs[i] }
func (a byPriority) Less(i, j int) bool {
	// All local specs will come first
	// WARNING: This will result in not following priorities for local APIServices.
	if a.specs[i].apiService.Spec.Service == nil {
		// Sort local specs with their name. This is the order in the delegation chain (aggregator first).
		return a.specs[i].apiService.Name < a.specs[j].apiService.Name
	}
	var iPriority, jPriority int32
	if a.specs[i].apiService.Spec.Group == a.specs[j].apiService.Spec.Group {
		iPriority = a.specs[i].apiService.Spec.VersionPriority
		jPriority = a.specs[i].apiService.Spec.VersionPriority
	} else {
		iPriority = a.groupPriorities[a.specs[i].apiService.Spec.Group]
		jPriority = a.groupPriorities[a.specs[j].apiService.Spec.Group]
	}
	if iPriority != jPriority {
		// Sort by priority, higher first
		return iPriority > jPriority
	}
	// Sort by service name.
	return a.specs[i].apiService.Name < a.specs[j].apiService.Name
}

func sortByPriority(specs []openAPISpecInfo) {
	b := byPriority{
		specs:           specs,
		groupPriorities: map[string]int32{},
	}
	for _, spec := range specs {
		if spec.apiService.Spec.Service == nil {
			continue
		}
		if pr, found := b.groupPriorities[spec.apiService.Spec.Group]; !found || spec.apiService.Spec.GroupPriorityMinimum > pr {
			b.groupPriorities[spec.apiService.Spec.Group] = spec.apiService.Spec.GroupPriorityMinimum
		}
	}
	sort.Sort(b)
}

// buildOpenAPISpec aggregates all OpenAPI specs.  It is not thread-safe. The caller is responsible to hold proper locks.
func (s *specAggregator) buildOpenAPISpec() (specToReturn *spec.Swagger, err error) {
	specs := []openAPISpecInfo{}
	for _, specInfo := range s.openAPISpecs {
		if specInfo.spec == nil {
			continue
		}
		specs = append(specs, *specInfo)
	}
	if len(specs) == 0 {
		return &spec.Swagger{}, nil
	}
	sortByPriority(specs)
	for _, specInfo := range specs {
		// TODO: Make kube-openapi.MergeSpec(s) accept nil or empty spec as destination and just clone the spec in that case.
		if specToReturn == nil {
			specToReturn = &spec.Swagger{}
			*specToReturn = *specInfo.spec
			// Paths and Definitions are set by MergeSpecsIgnorePathConflict
			specToReturn.Paths = nil
			specToReturn.Definitions = nil
		}
		if err := aggregator.MergeSpecsIgnorePathConflict(specToReturn, specInfo.spec); err != nil {
			return nil, err
		}
	}

	return specToReturn, nil
}

// updateOpenAPISpec aggregates all OpenAPI specs.  It is not thread-safe. The caller is responsible to hold proper locks.
func (s *specAggregator) updateOpenAPISpec() error {
	if s.openAPIVersionedService == nil {
		return nil
	}
	specToServe, err := s.buildOpenAPISpec()
	if err != nil {
		return err
	}
	return s.openAPIVersionedService.UpdateSpec(specToServe)
}

// tryUpdatingServiceSpecs tries updating openAPISpecs map with specified specInfo, and keeps the map intact
// if the update fails.
func (s *specAggregator) tryUpdatingServiceSpecs(specInfo *openAPISpecInfo) error {
	if specInfo == nil {
		return fmt.Errorf("invalid input: specInfo must be non-nil")
	}
	orgSpecInfo, exists := s.openAPISpecs[specInfo.apiService.Name]
	// Skip aggregation if OpenAPI spec didn't change
	if exists && orgSpecInfo != nil && orgSpecInfo.etag == specInfo.etag {
		return nil
	}
	s.openAPISpecs[specInfo.apiService.Name] = specInfo
	if err := s.updateOpenAPISpec(); err != nil {
		if exists {
			s.openAPISpecs[specInfo.apiService.Name] = orgSpecInfo
		} else {
			delete(s.openAPISpecs, specInfo.apiService.Name)
		}
		return err
	}
	return nil
}

// tryDeleteServiceSpecs tries delete specified specInfo from openAPISpecs map, and keeps the map intact
// if the update fails.
func (s *specAggregator) tryDeleteServiceSpecs(apiServiceName string) error {
	orgSpecInfo, exists := s.openAPISpecs[apiServiceName]
	if !exists {
		return nil
	}
	delete(s.openAPISpecs, apiServiceName)
	if err := s.updateOpenAPISpec(); err != nil {
		s.openAPISpecs[apiServiceName] = orgSpecInfo
		return err
	}
	return nil
}

// UpdateAPIServiceSpec updates the api service's OpenAPI spec. It is thread safe.
func (s *specAggregator) UpdateAPIServiceSpec(apiServiceName string, spec *spec.Swagger, etag string) error {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()

	specInfo, existingService := s.openAPISpecs[apiServiceName]
	if !existingService {
		return fmt.Errorf("APIService %q does not exists", apiServiceName)
	}

	// For APIServices (non-local) specs, only merge their /apis/ prefixed endpoint as it is the only paths
	// proxy handler delegates.
	if specInfo.apiService.Spec.Service != nil {
		spec = aggregator.FilterSpecByPathsWithoutSideEffects(spec, []string{"/apis/"})
	}

	return s.tryUpdatingServiceSpecs(&openAPISpecInfo{
		apiService: specInfo.apiService,
		spec:       spec,
		handler:    specInfo.handler,
		etag:       etag,
	})
}

// AddUpdateLocalAPIService allows adding/updating local API service with nil handler and
// nil Spec.Service. This function can be used for local dynamic OpenAPI spec aggregation
// management (e.g. CRD)
func (s *specAggregator) AddUpdateLocalAPIServiceSpec(name string, spec *spec.Swagger, etag string) error {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()

	localAPIService := apiregistration.APIService{
		Spec: apiregistration.APIServiceSpec{
			Service:              &apiregistration.ServiceReference{},
			Group:                "dynamiclocalgroup",
			Version:              "v1",
			GroupPriorityMinimum: 1000, // CRDs should have relatively low priority
			VersionPriority:      100,  // CRDs will be sorted by kube-like versions like any other APIService with the same VersionPriority
		},
	}
	localAPIService.Name = name
	return s.tryUpdatingServiceSpecs(&openAPISpecInfo{
		apiService: localAPIService,
		spec:       spec,
		etag:       etag,
	})
}

// AddUpdateAPIService adds or updates the api service. It is thread safe.
func (s *specAggregator) AddUpdateAPIService(handler http.Handler, apiService *apiregistration.APIService) error {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()

	if apiService.Spec.Service == nil {
		// All local specs should be already aggregated using local delegate chain
		return nil
	}

	newSpec := &openAPISpecInfo{
		apiService: *apiService,
		handler:    handler,
	}
	if specInfo, existingService := s.openAPISpecs[apiService.Name]; existingService {
		newSpec.etag = specInfo.etag
		newSpec.spec = specInfo.spec
	}
	return s.tryUpdatingServiceSpecs(newSpec)
}

// RemoveAPIServiceSpec removes an api service from OpenAPI aggregation. If it does not exist, no error is returned.
// It is thread safe.
func (s *specAggregator) RemoveAPIServiceSpec(apiServiceName string) error {
	s.rwMutex.Lock()
	defer s.rwMutex.Unlock()

	if _, existingService := s.openAPISpecs[apiServiceName]; !existingService {
		return nil
	}

	return s.tryDeleteServiceSpecs(apiServiceName)
}

// GetAPIServiceSpec returns api service spec info
func (s *specAggregator) GetAPIServiceInfo(apiServiceName string) (handler http.Handler, etag string, exists bool) {
	s.rwMutex.RLock()
	defer s.rwMutex.RUnlock()

	if info, existingService := s.openAPISpecs[apiServiceName]; existingService {
		return info.handler, info.etag, true
	}
	return nil, "", false
}
