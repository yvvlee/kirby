package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type invitationMembersFake struct {
	repository.ProjectMemberRepository
	role    string
	missing bool
}

func (m invitationMembersFake) GetForUpdate(context.Context, int64, int64) (*model.ProjectMember, error) {
	if m.missing {
		return nil, repository.Missing("member")
	}
	return &model.ProjectMember{Role: m.role}, nil
}
func TestInvitationRequiresCreatorCurrentAuthority(t *testing.T) {
	for _, tc := range []struct {
		name    string
		creator string
		grant   string
		missing bool
		allowed bool
	}{
		{name: "removed", missing: true, grant: "MEMBER"},
		{name: "demoted", creator: "MEMBER", grant: "GUEST"},
		{name: "owner demoted", creator: "MAINTAINER", grant: "OWNER"},
		{name: "maintainer equal", creator: "MAINTAINER", grant: "MAINTAINER", allowed: true},
		{name: "owner", creator: "OWNER", grant: "OWNER", allowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &ProjectInvitationService{members: invitationMembersFake{role: tc.creator, missing: tc.missing}}
			err := s.validCreator(context.Background(), &model.ProjectInvitation{ProjectID: 1, CreatedBy: 2, Role: tc.grant})
			if tc.allowed {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, permission.ErrForbidden)
		})
	}
}
