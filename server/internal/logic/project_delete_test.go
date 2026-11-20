package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type deleteProjects struct {
	repository.ProjectRepository
	deleted bool
}

func (r *deleteProjects) LockByID(context.Context, int64, int64) (*model.Project, error) {
	return &model.Project{}, nil
}
func (r *deleteProjects) Delete(context.Context, int64) error { r.deleted = true; return nil }

type deleteMembers struct {
	repository.ProjectMemberRepository
	role string
}

func (r deleteMembers) Get(context.Context, int64, int64) (*model.ProjectMember, error) {
	if r.role == "" {
		return nil, repository.Missing("member")
	}
	return &model.ProjectMember{Role: r.role}, nil
}

type deleteAudits struct{ repository.AuditLogRepository }

func (deleteAudits) RecordSystem(context.Context, *model.AuditLog) (int64, error) { return 1, nil }
func TestDeleteProjectOwnerOnly(t *testing.T) {
	for _, role := range []string{"OWNER", "MAINTAINER", "MEMBER", "GUEST", ""} {
		t.Run(role, func(t *testing.T) {
			projects := &deleteProjects{}
			logic := NewProjectLogicWithMembers(projects, nil, deleteAudits{}, matrixTX{}, deleteMembers{role: role})
			err := logic.Delete(context.Background(), permission.Actor{UserID: 2}, 9)
			if role == "OWNER" {
				if err != nil || !projects.deleted {
					t.Fatalf("owner deletion: %v", err)
				}
				return
			}
			if !errors.Is(err, permission.ErrForbidden) || projects.deleted {
				t.Fatalf("unauthorized deletion: %v", err)
			}
		})
	}
}
