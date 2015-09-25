package rfc2307

import (
	"github.com/go-ldap/ldap"

	"github.com/openshift/origin/pkg/auth/ldaputil"
)

// NewLDAPInterface builds a new LDAPInterface using a schema-appropriate config
func NewLDAPInterface(clientConfig ldaputil.LDAPClientConfig,
	groupQuery ldaputil.IdentifiyingLDAPQueryOptions,
	userQuery ldaputil.IdentifiyingLDAPQueryOptions,
	groupMembershipAttributes []string) LDAPInterface {
	return LDAPInterface{
		clientConfig:              clientConfig,
		groupQuery:                groupQuery,
		userQuery:                 userQuery,
		groupMembershipAttributes: groupMembershipAttributes,
		cachedUsers:               make(map[string]*ldap.Entry),
		cachedGroups:              make(map[string]*ldap.Entry),
	}
}

// LDAPInterface extracts the member list of an LDAP group entry from an LDAP server
// with first-class LDAP entries for groups. The LDAPInterface is *NOT* thread-safe.
// The LDAPInterface satisfies:
// - LDAPMemberExtractor
// - LDAPGroupGetter
// - LDAPGroupLister
type LDAPInterface struct {
	// clientConfig holds LDAP connection information
	clientConfig ldaputil.LDAPClientConfig
	// groupQuery holds the information necessary to make an LDAP query for a specific
	// first-class group entry on the LDAP server
	groupQuery ldaputil.IdentifiyingLDAPQueryOptions
	// userQuery holds the information necessary to make an LDAP query for a specific
	// first-class user entry on the LDAP server
	userQuery ldaputil.IdentifiyingLDAPQueryOptions
	// groupMembershipAttributes defines which attributes on an LDAP user entry will be interpreted
	// as the groups it is a member of
	groupMembershipAttributes []string

	// cachedGroups holds the result of group queries for later reference, indexed on group UID
	// e.g. this will map an LDAP group UID to the LDAP entry returned from the query made using it
	cachedGroups map[string]*ldap.Entry
	// cachedUsers holds the result of user queries for later reference, indexed on user UID
	// e.g. this will map an LDAP user UID to the LDAP entry returned from the query made using it
	cachedUsers map[string]*ldap.Entry
}

// ExtractMembers returns the LDAP member entries for a group specified with a ldapGroupUID
func (e *LDAPInterface) ExtractMembers(ldapGroupUID string) (members []*ldap.Entry, err error) {
	// get group entry from LDAP
	group, err := e.GroupEntryFor(ldapGroupUID)
	if err != nil {
		return nil, err
	}

	// extract member UIDs from group entry
	var ldapMemberUIDs []string
	for _, attribute := range e.userQuery.NameAttributes {
		ldapMemberUIDs = append(ldapMemberUIDs, group.GetAttributeValues(attribute)...)
	}

	// find members on LDAP server or in cache
	for _, ldapMemberUID := range ldapMemberUIDs {
		memberEntry, err := e.userEntryFor(ldapMemberUID)
		if err != nil {
			return nil, err
		}
		members = append(members, memberEntry)
	}
	return members, nil
}

// GroupFor returns an LDAP group entry for the given group UID by searching the internal cache
// of the LDAPInterface first, then sending an LDAP query if the cache did not contain the entry.
// This also satisfies the LDAPGroupGetter interface
func (e *LDAPInterface) GroupEntryFor(ldapGroupUID string) (group *ldap.Entry, err error) {
	group, exists := e.cachedGroups[ldapGroupUID]
	if !exists {
		group, err = e.queryForGroup(ldapGroupUID)
		if err != nil {
			return nil, err
		}
		// cache for annotation extraction
		e.cachedGroups[ldapGroupUID] = group
	}
	return group, nil
}

// queryForGroup queries for a specific group identified by a ldapGroupUID with the query config stored
// in a LDAPInterface
func (e *LDAPInterface) queryForGroup(ldapGroupUID string) (group *ldap.Entry, err error) {
	// create the search request
	searchRequest, err := e.groupQuery.NewSearchRequest(ldapGroupUID, e.groupMembershipAttributes)
	if err != nil {
		return nil, err
	}

	return ldaputil.QueryForUniqueEntry(e.clientConfig, searchRequest)
}

// userEntryFor returns an LDAP group entry for the given group UID by searching the internal cache
// of the LDAPInterface first, then sending an LDAP query if the cache did not contain the entry
func (e *LDAPInterface) userEntryFor(ldapUserUID string) (user *ldap.Entry, err error) {
	user, exists := e.cachedUsers[ldapUserUID]
	if !exists {
		user, err = e.queryForUser(ldapUserUID)
		if err != nil {
			return nil, err
		}
		// cache for annotation extraction
		e.cachedUsers[ldapUserUID] = user
	}
	return user, nil
}

// queryForUser queries for an LDAP user entry identified with an LDAP user UID on an LDAP server
// determined from a clientConfig by creating a search request from an LDAP query template and
// determining which attributes to search for with a LDAPuserAttributeDefiner
func (e *LDAPInterface) queryForUser(ldapUserUID string) (user *ldap.Entry, err error) {
	// create the search request
	searchRequest, err := e.userQuery.NewSearchRequest(ldapUserUID, []string{})
	if err != nil {
		return nil, err
	}

	return ldaputil.QueryForUniqueEntry(e.clientConfig, searchRequest)
}

// ListGroups queries for all groups as configured with the common group filter and returns their
// LDAP group UIDs. This also satisfies the LDAPGroupLister interface
func (e *LDAPInterface) ListGroups() (ldapGroupUIDs []string, err error) {
	groups, err := e.queryForGroups()
	if err != nil {
		return nil, err
	}
	for _, group := range groups {
		// cache groups returned from the server for later
		ldapGroupUID := ldaputil.GetAttributeValue(group, e.groupQuery.NameAttributes)
		e.cachedGroups[ldapGroupUID] = group
		ldapGroupUIDs = append(ldapGroupUIDs, ldapGroupUID)
	}
	return ldapGroupUIDs, nil
}

// queryForGroups queries for all groups identified by a common filter in the query config stored
// in a GroupListerDataExtractor
func (e *LDAPInterface) queryForGroups() (groups []*ldap.Entry, err error) {
	// create the search request
	searchRequest := e.groupQuery.LDAPQuery.NewSearchRequest(e.groupMembershipAttributes)
	return ldaputil.QueryForEntries(e.clientConfig, searchRequest)
}
