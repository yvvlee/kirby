package service

import (
	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"testing"
)

func TestMatrixValidation(t *testing.T) {
	cases := []struct {
		name    string
		roles   []*adminv1.EnvironmentPermissionRole
		invalid bool
	}{
		{"tighten", []*adminv1.EnvironmentPermissionRole{{Role: "MEMBER", Permissions: []string{}}}, false},
		{"open", []*adminv1.EnvironmentPermissionRole{{Role: "GUEST", Permissions: []string{"READ", "WRITE"}}}, false},
		{"unknown role", []*adminv1.EnvironmentPermissionRole{{Role: "ADMIN"}}, true},
		{"owner restriction", []*adminv1.EnvironmentPermissionRole{{Role: "OWNER", Permissions: []string{"READ"}}}, true},
		{"unknown point", []*adminv1.EnvironmentPermissionRole{{Role: "MEMBER", Permissions: []string{"DELETE"}}}, true},
		{"duplicate point", []*adminv1.EnvironmentPermissionRole{{Role: "MEMBER", Permissions: []string{"READ", "READ"}}}, true},
		{"duplicate role", []*adminv1.EnvironmentPermissionRole{{Role: "MEMBER"}, {Role: "MEMBER"}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if (validateMatrix(tc.roles) != nil) != tc.invalid {
				t.Fatal("unexpected validation result")
			}
		})
	}
}
