// Copyright 2016 Documize Inc. <legal@documize.com>. All rights reserved.
//
// This software (Documize Community Edition) is licensed under
// GNU AGPL v3 http://www.gnu.org/licenses/agpl-3.0.en.html
//
// You can operate outside the AGPL restrictions by purchasing
// Documize Enterprise Edition and obtaining a commercial license
// by contacting <sales@documize.com>.
//
// https://documize.com

package ldap

import (
	"crypto/tls"
	"fmt"
	"strings"
	"testing"

	lm "github.com/documize/community/model/auth"
	ld "gopkg.in/ldap.v2"
)

// Works against https://github.com/rroemhild/docker-test-openldap
// Use docker run --privileged -d -p 389:389 rroemhild/test-openldap

func TestLocalLDAPServer_AllUsers(t *testing.T) {
	c := lm.LDAPConfig{}
	c.ServerHost = "127.0.0.1"
	c.ServerPort = 389
	c.EncryptionType = "starttls"
	c.BaseDN = "ou=people,dc=planetexpress,dc=com"
	c.BindDN = "cn=admin,dc=planetexpress,dc=com"
	c.BindPassword = "GoodNewsEveryone"
	c.UserFilter = ""
	c.GroupFilter = ""

	address := fmt.Sprintf("%s:%d", c.ServerHost, c.ServerPort)

	t.Log("Connecting to LDAP server", address)

	l, err := ld.Dial("tcp", address)
	if err != nil {
		t.Error("Error: unable to dial LDAP server: ", err.Error())
		return
	}
	defer l.Close()

	if c.EncryptionType == "starttls" {
		t.Log("Using StartTLS with LDAP server")
		err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
		if err != nil {
			t.Error("Error: unable to startTLS with LDAP server: ", err.Error())
			return
		}
	}

	// Authenticate with LDAP server using admin credentials.
	t.Log("Binding LDAP admin user")
	err = l.Bind(c.BindDN, c.BindPassword)
	if err != nil {
		t.Error("Error: unable to bind specified admin user to LDAP: ", err.Error())
		return
	}

	// Get users from LDAP server by using filter
	filter := ""
	attrs := []string{}
	if len(c.GroupFilter) > 0 {
		filter = c.GroupFilter
		attrs = []string{"dn", "cn"}
	} else if len(c.UserFilter) > 0 {
		filter = c.UserFilter
		attrs = []string{"dn", "cn", "givenName", "sn", "mail", "uid"}
	} else {
		filter = "(|(objectClass=person)(objectClass=user)(objectClass=inetOrgPerson))"
		attrs = []string{"dn", "cn", "givenName", "sn", "mail", "uid"}
	}

	searchRequest := ld.NewSearchRequest(
		c.BaseDN,
		ld.ScopeWholeSubtree, ld.NeverDerefAliases, 0, 0, false,
		filter,
		attrs,
		nil,
	)

	t.Log("LDAP search filter:", filter)
	sr, err := l.Search(searchRequest)
	if err != nil {
		t.Error("Error: unable to execute directory search: ", err.Error())
		return
	}

	t.Logf("LDAP search entries found: %d", len(sr.Entries))
	if len(sr.Entries) == 0 {
		t.Error("Received ZERO LDAP search entries")
		return
	}

	for _, entry := range sr.Entries {
		t.Logf("[%s] %s (%s %s) @ %s\n",
			entry.GetAttributeValue("uid"),
			entry.GetAttributeValue("cn"),
			entry.GetAttributeValue("givenName"),
			entry.GetAttributeValue("sn"),
			entry.GetAttributeValue("mail"))
	}
}

func TestLocalLDAPServer_UsersInGroup(t *testing.T) {
	c := lm.LDAPConfig{}
	c.ServerHost = "127.0.0.1"
	c.ServerPort = 389
	c.EncryptionType = "starttls"
	c.BaseDN = "dc=planetexpress,dc=com"
	c.BindDN = "cn=admin,dc=planetexpress,dc=com"
	c.BindPassword = "GoodNewsEveryone"
	c.UserFilter = ""
	c.GroupFilter = "(&(objectClass=group)(cn=ship_crew))"

	address := fmt.Sprintf("%s:%d", c.ServerHost, c.ServerPort)

	t.Log("Connecting to LDAP server", address)

	l, err := ld.Dial("tcp", address)
	if err != nil {
		t.Error("Error: unable to dial LDAP server: ", err.Error())
		return
	}
	defer l.Close()

	if c.EncryptionType == "starttls" {
		t.Log("Using StartTLS with LDAP server")
		err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
		if err != nil {
			t.Error("Error: unable to startTLS with LDAP server: ", err.Error())
			return
		}
	}

	// Authenticate with LDAP server using admin credentials.
	t.Log("Binding LDAP admin user")
	err = l.Bind(c.BindDN, c.BindPassword)
	if err != nil {
		t.Error("Error: unable to bind specified admin user to LDAP: ", err.Error())
		return
	}

	// Get users from LDAP server by using filter
	filter := ""
	attrs := []string{}
	if len(c.GroupFilter) > 0 {
		filter = c.GroupFilter
		attrs = []string{"cn", "member"}
	} else if len(c.UserFilter) > 0 {
		filter = c.UserFilter
		attrs = []string{"dn", "cn", "givenName", "sn", "mail", "uid"}
	} else {
		filter = "(|(objectCategory=person)(objectClass=person)(objectClass=user)(objectClass=inetOrgPerson))"
		attrs = []string{"dn", "cn", "givenName", "sn", "mail", "uid"}
	}

	searchRequest := ld.NewSearchRequest(
		c.BaseDN,
		ld.ScopeWholeSubtree, ld.NeverDerefAliases, 0, 0, false,
		filter,
		attrs,
		nil,
	)

	t.Log("LDAP search filter:", filter)
	sr, err := l.Search(searchRequest)
	if err != nil {
		t.Error("Error: unable to execute directory search: ", err.Error())
		return
	}

	t.Logf("LDAP search entries found: %d", len(sr.Entries))
	if len(sr.Entries) == 0 {
		t.Error("Received ZERO LDAP search entries")
		return
	}

	// Get list of group members
	rawMembers := sr.Entries[0].GetAttributeValues("member")

	if len(rawMembers) == 0 {
		t.Error("Error: group member attribute returned no users")
		return
	}

	t.Logf("LDAP group contains %d members", len(rawMembers))

	for _, entry := range rawMembers {
		// get CN element from DN
		parts := strings.Split(entry, ",")
		if len(parts) == 0 {
			continue
		}
		filter := fmt.Sprintf("(%s)", parts[0])

		usr := ld.NewSearchRequest(
			c.BaseDN,
			ld.ScopeWholeSubtree, ld.NeverDerefAliases, 0, 0, false,
			filter,
			[]string{"dn", "cn", "givenName", "sn", "mail", "uid"},
			nil,
		)
		ue, err := l.Search(usr)
		if err != nil {
			t.Error("Error: unable to execute directory search for group member: ", err.Error())
			return
		}

		if len(ue.Entries) > 0 {
			for _, ur := range ue.Entries {
				t.Logf("%s", ur.GetAttributeValue("mail"))
			}
		} else {
			t.Log("group member search failed:", filter)
		}
	}
}
func TestLocalLDAP_Authenticate(t *testing.T) {
	c := lm.LDAPConfig{}
	c.ServerHost = "127.0.0.1"
	c.ServerPort = 389
	c.EncryptionType = "starttls"
	c.BaseDN = "ou=people,dc=planetexpress,dc=com"
	c.BindDN = "cn=admin,dc=planetexpress,dc=com"
	c.BindPassword = "GoodNewsEveryone"
	c.UserFilter = ""
	c.GroupFilter = ""

	address := fmt.Sprintf("%s:%d", c.ServerHost, c.ServerPort)
	t.Log("Connecting to LDAP server", address)
	l, err := ld.Dial("tcp", address)
	if err != nil {
		t.Error("Error: unable to dial AD server: ", err.Error())
		return
	}
	defer l.Close()

	if c.EncryptionType == "starttls" {
		t.Log("Using StartTLS with LDAP server")
		err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
		if err != nil {
			t.Error("Error: unable to startTLS with LDAP server: ", err.Error())
			return
		}
	}

	// Authenticate with LDAP server using admin credentials.
	t.Log("Binding LDAP admin user")
	err = l.Bind(c.BindDN, c.BindPassword)
	if err != nil {
		t.Error("Error: unable to bind specified admin user to LDAP: ", err.Error())
		return
	}

	username := "professor"
	password := "professor"
	filter := fmt.Sprintf("(uid=%s)", username)

	searchRequest := ld.NewSearchRequest(
		c.BaseDN,
		ld.ScopeWholeSubtree, ld.NeverDerefAliases, 0, 0, false,
		filter,
		[]string{"mail"},
		nil,
	)

	t.Log("LDAP search filter:", filter)
	sr, err := l.Search(searchRequest)
	if err != nil {
		t.Error("Error: unable to execute directory search: ", err.Error())
		return
	}

	if len(sr.Entries) == 0 {
		t.Error("Error: user not found")
		return
	}
	if len(sr.Entries) != 1 {
		t.Error("Error: too many users found during authentication")
		return
	}

	userdn := sr.Entries[0].DN

	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		t.Error("Error: invalid credentials", err.Error())
		return
	}

	t.Log("Authenticated", username)
}
