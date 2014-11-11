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

package labels

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// Selector represents a label selector.
type Selector interface {
	// Matches returns true if this selector matches the given set of labels.
	Matches(Labels) bool

	// Empty returns true if this selector does not restrict the selection space.
	Empty() bool

	// RequiresExactMatch allows a caller to introspect whether a given selector
	// requires a single specific label to be set, and if so returns the value it
	// requires.
	// TODO: expand this to be more general
	RequiresExactMatch(label string) (value string, found bool)

	// String returns a human readable string that represents this selector.
	String() string
}

// Everything returns a selector that matches all labels.
func Everything() Selector {
	return andTerm{}
}

type hasTerm struct {
	label, value string
}

func (t *hasTerm) Matches(ls Labels) bool {
	return ls.Get(t.label) == t.value
}

func (t *hasTerm) Empty() bool {
	return false
}

func (t *hasTerm) RequiresExactMatch(label string) (value string, found bool) {
	if t.label == label {
		return t.value, true
	}
	return "", false
}

func (t *hasTerm) String() string {
	return fmt.Sprintf("%v=%v", t.label, t.value)
}

type notHasTerm struct {
	label, value string
}

func (t *notHasTerm) Matches(ls Labels) bool {
	return ls.Get(t.label) != t.value
}

func (t *notHasTerm) Empty() bool {
	return false
}

func (t *notHasTerm) RequiresExactMatch(label string) (value string, found bool) {
	return "", false
}

func (t *notHasTerm) String() string {
	return fmt.Sprintf("%v!=%v", t.label, t.value)
}

type andTerm []Selector

func (t andTerm) Matches(ls Labels) bool {
	for _, q := range t {
		if !q.Matches(ls) {
			return false
		}
	}
	return true
}

func (t andTerm) Empty() bool {
	if t == nil {
		return true
	}
	if len([]Selector(t)) == 0 {
		return true
	}
	for i := range t {
		if !t[i].Empty() {
			return false
		}
	}
	return true
}

func (t andTerm) RequiresExactMatch(label string) (string, bool) {
	if t == nil || len([]Selector(t)) == 0 {
		return "", false
	}
	for i := range t {
		if value, found := t[i].RequiresExactMatch(label); found {
			return value, found
		}
	}
	return "", false
}

func (t andTerm) String() string {
	var terms []string
	for _, q := range t {
		terms = append(terms, q.String())
	}
	return strings.Join(terms, ",")
}

// Operator represents a key's relationship
// to a set of values in a Requirement.
type Operator int

const (
	In Operator = iota + 1
	NotIn
	Exists
)

// LabelSelector contains a list of Requirements.
// LabelSelector is set-based and is distinguished from exact
// match-based selectors composed of key=value matching conjunctions.
// TODO: Remove previous sentence when exact match-based
// selectors are removed.
type LabelSelector struct {
	Requirements []Requirement
}

// Requirement is a selector that contains values, a key
// and an operator that relates the key and values. The zero
// value of Requirement is invalid. See the NewRequirement
// constructor for creating a valid Requirement.
// Requirement is set-based and is distinguished from exact
// match-based selectors composed of key=value matching.
// TODO: Remove previous sentence when exact match-based
// selectors are removed.
type Requirement struct {
	key       string
	operator  Operator
	strValues util.StringSet
}

// NewRequirement is the constructor for a Requirement.
// If either of these rules is violated, an error is returned:
// (1) The operator can only be In, NotIn or Exists.
// (2) If the operator is In or NotIn, the values set must
//     be non-empty.
//
// The empty string is a valid value in the input values set.
func NewRequirement(key string, op Operator, vals util.StringSet) (*Requirement, error) {
	switch op {
	case In, NotIn:
		if len(vals) == 0 {
			return nil, fmt.Errorf("for In,NotIn operators, values set can't be empty")
		}
	case Exists:
	default:
		return nil, fmt.Errorf("operator '%v' is not recognized", op)
	}
	return &Requirement{key: key, operator: op, strValues: vals}, nil
}

// Matches returns true if the Requirement matches the input Labels.
// There is a match in the following cases:
// (1) The operator is Exists and Labels has the Requirement's key.
// (2) The operator is In, Labels has the Requirement's key and Labels'
//     value for that key is in Requirement's value set.
// (3) The operator is NotIn, Labels has the Requirement's key and
//     Labels' value for that key is not in Requirement's value set.
// (4) The operator is NotIn and Labels does not have the
//     Requirement's key.
//
// If called on an invalid Requirement, an error is returned. See
// NewRequirement for creating a valid Requirement.
func (r *Requirement) Matches(ls Labels) (bool, error) {
	switch r.operator {
	case In:
		if !ls.Has(r.key) {
			return false, nil
		}
		return r.strValues.Has(ls.Get(r.key)), nil
	case NotIn:
		if !ls.Has(r.key) {
			return true, nil
		}
		return !r.strValues.Has(ls.Get(r.key)), nil
	case Exists:
		return ls.Has(r.key), nil
	default:
		return false, fmt.Errorf("requirement is not set: %+v", r)
	}
}

// String returns a human-readable string that represents this
// Requirement. If called on an invalid Requirement, an error is
// returned. See NewRequirement for creating a valid Requirement.
func (r *Requirement) String() (string, error) {
	var buffer bytes.Buffer
	buffer.WriteString(r.key)

	switch r.operator {
	case In:
		buffer.WriteString(" in ")
	case NotIn:
		buffer.WriteString(" not in ")
	case Exists:
		return buffer.String(), nil
	default:
		return "", fmt.Errorf("requirement is not set: %+v", r)
	}

	buffer.WriteString("(")
	if len(r.strValues) == 1 {
		buffer.WriteString(r.strValues.List()[0])
	} else { // only > 1 since == 0 prohibited by NewRequirement
		buffer.WriteString(strings.Join(r.strValues.List(), ","))
	}
	buffer.WriteString(")")
	return buffer.String(), nil
}

// Matches for a LabelSelector returns true if all
// its Requirements match the input Labels. If any
// Requirement does not match, false is returned.
// An error is returned if any match attempt between
// a Requirement and the input Labels returns an error.
func (lsel *LabelSelector) Matches(l Labels) (bool, error) {
	for _, req := range lsel.Requirements {
		if matches, err := req.Matches(l); err != nil {
			return false, err
		} else if !matches {
			return false, nil
		}
	}
	return true, nil
}

// String returns a comma-separated string of all
// the LabelSelector Requirements' human-readable strings.
// An error is returned if any attempt to get a
// Requirement's  human-readable string returns an error.
func (lsel *LabelSelector) String() (string, error) {
	var reqs []string
	for _, req := range lsel.Requirements {
		if str, err := req.String(); err != nil {
			return "", err
		} else {
			reqs = append(reqs, str)
		}
	}
	return strings.Join(reqs, ","), nil
}

// TODO: Parse takes a string representing a selector and returns
// a selector, or an error. A well-formed input string follows
// the syntax of that which is returned by LabelSelector.String
// and therefore is largely controlled by that which is returned
// by Requirement.String. The returned selector object's type
// should be an interface implemented by LabelSelector. Note that
// this parsing function is different than ParseSelector since
// they parse different selectors with different syntaxes.
// See comments above for LabelSelector and Requirement struct
// definition for more details.

func try(selectorPiece, op string) (lhs, rhs string, ok bool) {
	pieces := strings.Split(selectorPiece, op)
	if len(pieces) == 2 {
		return pieces[0], pieces[1], true
	}
	return "", "", false
}

// SelectorFromSet returns a Selector which will match exactly the given Set. A
// nil Set is considered equivalent to Everything().
func SelectorFromSet(ls Set) Selector {
	if ls == nil {
		return Everything()
	}
	items := make([]Selector, 0, len(ls))
	for label, value := range ls {
		items = append(items, &hasTerm{label: label, value: value})
	}
	if len(items) == 1 {
		return items[0]
	}
	return andTerm(items)
}

// ParseSelector takes a string representing a selector and returns an
// object suitable for matching, or an error.
func ParseSelector(selector string) (Selector, error) {
	parts := strings.Split(selector, ",")
	sort.StringSlice(parts).Sort()
	var items []Selector
	for _, part := range parts {
		if part == "" {
			continue
		}
		if lhs, rhs, ok := try(part, "!="); ok {
			items = append(items, &notHasTerm{label: lhs, value: rhs})
		} else if lhs, rhs, ok := try(part, "=="); ok {
			items = append(items, &hasTerm{label: lhs, value: rhs})
		} else if lhs, rhs, ok := try(part, "="); ok {
			items = append(items, &hasTerm{label: lhs, value: rhs})
		} else {
			return nil, fmt.Errorf("invalid selector: '%s'; can't understand '%s'", selector, part)
		}
	}
	if len(items) == 1 {
		return items[0], nil
	}
	return andTerm(items), nil
}
