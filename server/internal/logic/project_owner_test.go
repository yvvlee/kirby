package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/yvvlee/kirby/server/internal/permission"
)

func TestProjectOwnerRequirement(t *testing.T) {
	for _, role := range []string{"MAINTAINER", "MEMBER", "GUEST", "UNKNOWN"} {
		t.Run(role, func(t *testing.T) {
			l := &ProjectLogic{
				members: memberRejectRepo{
					actor: role,
				},
			}
			if err := l.requireOwner(context.Background(), 7, 1); !errors.Is(err, permission.ErrForbidden) {
				t.Fatalf("expected forbidden for %s, got %v", role, err)
			}
		})
	}
	l := &ProjectLogic{
		members: memberRejectRepo{
			actor: "OWNER",
		},
	}
	if err := l.requireOwner(context.Background(), 7, 1); err != nil {
		t.Fatal(err)
	}
	if err := l.requireOwner(context.Background(), 7, 2); err == nil {
		t.Fatal("cross-project owner accepted")
	}
}
