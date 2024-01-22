package models

import (
	"fmt"
	log "github.com/gophish/gophish/logger"

	"gopkg.in/check.v1"
)

type PermissionCheck map[string]bool

func (s *ModelsSuite) TestHasPermission(c *check.C) {

	permissionTests := map[string]PermissionCheck{
		RoleAdmin: {
			PermissionModifySystem:  true,
			PermissionModifyObjects: true,
			PermissionViewObjects:   true,
		},
		RoleUser: {
			PermissionModifySystem:  false,
			PermissionModifyObjects: true,
			PermissionViewObjects:   true,
		},
	}

	for r, checks := range permissionTests {
		// Create the user with the provided role
		role, err := GetRoleBySlug(r)
		c.Assert(err, check.Equals, nil)
		user := User{
			Username: fmt.Sprintf("test-%s", r),
			Hash:     "12345",
			ApiKey:   fmt.Sprintf("%s-key", r),
			RoleID:   role.ID,
		}
		err = PutUser(&user)
		if err != nil {
			log.Error(err)
		}

		// Perform the permission checks
		for permission, expected := range checks {
			access, err := user.HasPermission(permission)
			fmt.Printf("Checking %s -> %s\n", r, permission)
			c.Assert(err, check.Equals, nil)
			c.Assert(access, check.Equals, expected)
		}
	}
}

func (s *ModelsSuite) TestGetRoleBySlug(c *check.C) {
	roles := []string{RoleAdmin, RoleUser}
	for _, role := range roles {
		got, err := GetRoleBySlug(role)
		c.Assert(err, check.Equals, nil)
		c.Assert(got.Slug, check.Equals, role)
	}
	_, err := GetRoleBySlug("bogus")
	c.Assert(err, check.NotNil)
}
