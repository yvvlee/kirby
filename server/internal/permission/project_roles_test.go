package permission

import "testing"

func TestDefaultEnvironmentPermissions(t *testing.T) {
	tests := []struct {
		role ProjectRole
		want int
	}{
		{role: ProjectRoleOwner, want: 4},
		{role: ProjectRoleMaintainer, want: 4},
		{role: ProjectRoleMember, want: 2},
		{role: ProjectRoleGuest, want: 1},
	}
	for _, test := range tests {
		if got := len(DefaultEnvironmentPermissions(test.role)); got != test.want {
			t.Fatalf("role %s has %d permissions, want %d", test.role, got, test.want)
		}
	}
}

func TestCanGrant(t *testing.T) {
	if !CanGrant(ProjectRoleOwner, ProjectRoleMaintainer) {
		t.Fatal("owner should grant maintainer")
	}
	if CanGrant(ProjectRoleMaintainer, ProjectRoleOwner) {
		t.Fatal("maintainer should not grant owner")
	}
	if !CanGrant(ProjectRoleMaintainer, ProjectRoleGuest) {
		t.Fatal("maintainer should grant guest")
	}
}

func TestCanGrantRejectsNonManagersAndUnknownRoles(t *testing.T) {
	for _, actor := range []ProjectRole{ProjectRoleMember, ProjectRoleGuest, "UNKNOWN"} {
		for _, target := range []ProjectRole{ProjectRoleOwner, ProjectRoleMaintainer, ProjectRoleMember, ProjectRoleGuest} {
			if CanGrant(actor, target) {
				t.Errorf("%s must not manage %s", actor, target)
			}
		}
	}
	for _, actor := range []ProjectRole{ProjectRoleOwner, ProjectRoleMaintainer} {
		if CanGrant(actor, "UNKNOWN") || CanGrant(actor, "") {
			t.Errorf("%s must not grant an unknown role", actor)
		}
	}
}
