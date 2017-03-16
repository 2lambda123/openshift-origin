package cache

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/openshift/origin/pkg/authorization/authorizer"
)

func TestAuthorizer(t *testing.T) {
	_, _ = NewAuthorizer(nil, time.Minute, 1000)
}

func TestCacheKey(t *testing.T) {
	tests := map[string]struct {
		Context apirequest.Context
		Attrs   authorizer.Action

		ExpectedKey string
		ExpectedErr bool
	}{
		"uncacheable request attributes": {
			Context:     apirequest.NewContext(),
			Attrs:       &authorizer.DefaultAuthorizationAttributes{RequestAttributes: true},
			ExpectedErr: true,
		},
		"empty": {
			Context:     apirequest.NewContext(),
			Attrs:       &authorizer.DefaultAuthorizationAttributes{},
			ExpectedKey: `{"apiGroup":"","apiVersion":"","nonResourceURL":false,"resource":"","resourceName":"","url":"","verb":""}`,
		},
		"full": {
			Context: apirequest.WithUser(apirequest.WithNamespace(apirequest.NewContext(), "myns"), &user.DefaultInfo{Name: "me", Groups: []string{"group1", "group2"}}),
			Attrs: &authorizer.DefaultAuthorizationAttributes{
				Verb:              "v",
				APIVersion:        "av",
				APIGroup:          "ag",
				Resource:          "r",
				ResourceName:      "rn",
				RequestAttributes: nil,
				NonResourceURL:    true,
				URL:               "/abc",
			},
			ExpectedKey: `{"apiGroup":"ag","apiVersion":"av","groups":["group1","group2"],"namespace":"myns","nonResourceURL":true,"resource":"r","resourceName":"rn","scopes":null,"url":"/abc","user":"me","verb":"v"}`,
		},
	}

	for k, tc := range tests {
		key, err := cacheKey(tc.Context, tc.Attrs)
		if tc.ExpectedErr != (err != nil) {
			t.Errorf("%s: expected err=%v, got %v", k, tc.ExpectedErr, err)
		}
		if tc.ExpectedKey != key {
			t.Errorf("%s: expected key=%v, got %v", k, tc.ExpectedKey, key)
		}
	}
}

func TestCacheKeyFields(t *testing.T) {
	keyJSON, err := cacheKey(apirequest.NewContext(), &authorizer.DefaultAuthorizationAttributes{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	keyMap := map[string]interface{}{}
	if err := json.Unmarshal([]byte(keyJSON), &keyMap); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	keys := sets.NewString()
	for k := range keyMap {
		keys.Insert(strings.ToLower(k))
	}

	// These are results we don't expect to be in the cache key
	expectedMissingKeys := sets.NewString("requestattributes")

	attrType := reflect.TypeOf((*authorizer.Action)(nil)).Elem()
	for i := 0; i < attrType.NumMethod(); i++ {
		name := attrType.Method(i).Name
		name = strings.TrimPrefix(name, "Get")
		name = strings.TrimPrefix(name, "Is")
		name = strings.ToLower(name)
		if !keys.Has(name) && !expectedMissingKeys.Has(name) {
			t.Errorf("computed cache is missing an entry for %s", attrType.Method(i).Name)
		}
	}
}
