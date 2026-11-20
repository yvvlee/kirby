package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	adminv1 "github.com/yvvlee/kirby/server/api/admin"
	"github.com/yvvlee/kirby/server/internal/logic"
	"github.com/yvvlee/kirby/server/internal/model"
	"github.com/yvvlee/kirby/server/internal/permission"
	"github.com/yvvlee/kirby/server/internal/repository"
)

type ProjectInvitationService struct {
	projects     repository.ProjectRepository
	invitations  repository.ProjectInvitationRepository
	members      repository.ProjectMemberRepository
	settings     repository.ApplicationSettingsRepository
	transactions logic.TransactionRunner
}

// NewProjectInvitationService constructs invitation operations.
// Parameters: invitations persists tokens; members persists roles; settings gates links; transactions commits acceptance atomically.
// Returns: the configured service.
// Throws: none.
func NewProjectInvitationService(invitations repository.ProjectInvitationRepository, members repository.ProjectMemberRepository, settings repository.ApplicationSettingsRepository, transactions logic.TransactionRunner, projects repository.ProjectRepository) *ProjectInvitationService {
	return &ProjectInvitationService{
		invitations:  invitations,
		projects:     projects,
		members:      members,
		settings:     settings,
		transactions: transactions,
	}
}

func (s *ProjectInvitationService) authorize(ctx context.Context, projectID int64, target string) (int64, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return 0, err
	}
	member, err := s.members.Get(ctx, projectID, actor.UserID)
	if err != nil {
		return 0, permission.ErrForbidden
	}
	if member.Role != "OWNER" && member.Role != "MAINTAINER" {
		return 0, permission.ErrForbidden
	}
	if !permission.CanGrant(permission.ProjectRole(member.Role), permission.ProjectRole(target)) {
		return 0, permission.ErrForbidden
	}
	return actor.UserID, nil
}

// CreateProjectInvitation generates a single-use link.
// Parameters: ctx authenticates the actor; req selects project and role.
// Returns: public invitation details and the one-time token.
// Throws: authorization or persistence errors.
func (s *ProjectInvitationService) CreateProjectInvitation(ctx context.Context, req *adminv1.CreateProjectInvitationRequest) (*adminv1.ProjectInvitationReply, error) {
	actorID, err := s.authorize(ctx, req.ProjectId, req.Role)
	if err != nil {
		return nil, apiError(err)
	}
	settings, err := s.settings.Get(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	if !settings.AllowSelfRegistration {
		return nil, apiError(permission.ErrForbidden)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, apiError(err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	item := &model.ProjectInvitation{
		ProjectID: req.ProjectId,
		Role:      req.Role,
		TokenHash: hash[:],
		CreatedBy: actorID,
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
	}
	if err := s.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := s.projects.LockByID(txctx, 0, item.ProjectID); err != nil {
			return err
		}
		if _, err := s.authorizeLocked(txctx, item.ProjectID, item.Role); err != nil {
			return err
		}
		return s.invitations.Create(txctx, item)
	}); err != nil {
		return nil, apiError(err)
	}
	reply := invitationReply(item)
	reply.Token = token
	return reply, nil
}

func (s *ProjectInvitationService) active(ctx context.Context, token string) (*model.ProjectInvitation, error) {
	hash := sha256.Sum256([]byte(token))
	return s.invitations.GetActive(ctx, hash[:], time.Now().UTC())
}

// GetProjectInvitation previews a link without joining.
// Parameters: ctx controls reads; req contains the bearer invitation token.
// Returns: invitation details.
// Throws: expired, used, revoked or storage errors.
func (s *ProjectInvitationService) GetProjectInvitation(ctx context.Context, req *adminv1.ProjectInvitationTokenRequest) (*adminv1.ProjectInvitationReply, error) {
	item, err := s.active(ctx, req.Token)
	if err != nil {
		return nil, apiError(err)
	}
	return invitationReply(item), nil
}

// AcceptProjectInvitation consumes a token and creates membership atomically.
// Parameters: ctx identifies the accepting user; req contains the invitation token.
// Returns: accepted invitation details.
// Throws: invalid invitation, authorization or persistence errors.
func (s *ProjectInvitationService) AcceptProjectInvitation(ctx context.Context, req *adminv1.ProjectInvitationTokenRequest) (*adminv1.ProjectInvitationReply, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	var item *model.ProjectInvitation
	err = s.transactions.TX(ctx, func(txctx context.Context) error {
		var err error
		item, err = s.active(txctx, req.Token)
		if err != nil {
			return err
		}
		if _, err := s.projects.LockByID(txctx, 0, item.ProjectID); err != nil {
			return err
		}
		if err := s.validCreator(txctx, item); err != nil {
			return err
		}
		if err := s.invitations.Accept(txctx, item.ID, actor.UserID, time.Now().UTC()); err != nil {
			return err
		}
		return s.members.Add(txctx, &model.ProjectMember{
			ProjectID: item.ProjectID,
			UserID:    actor.UserID,
			Role:      item.Role,
			CreatedBy: item.CreatedBy,
		})
	})
	if err != nil {
		return nil, apiError(err)
	}
	return invitationReply(item), nil
}

// RevokeProjectInvitation invalidates an invitation immediately.
// Parameters: ctx identifies the manager; req contains the invitation token.
// Returns: an empty acknowledgement.
// Throws: authorization or persistence errors.
func (s *ProjectInvitationService) RevokeProjectInvitation(ctx context.Context, req *adminv1.ProjectInvitationTokenRequest) (*emptypb.Empty, error) {
	item, err := s.active(ctx, req.Token)
	if err != nil {
		return nil, apiError(err)
	}
	if _, err := s.authorize(ctx, item.ProjectID, item.Role); err != nil {
		return nil, apiError(err)
	}
	if err := s.transactions.TX(ctx, func(txctx context.Context) error {
		if _, err := s.projects.LockByID(txctx, 0, item.ProjectID); err != nil {
			return err
		}
		if _, err := s.authorizeLocked(txctx, item.ProjectID, item.Role); err != nil {
			return err
		}
		return s.invitations.Revoke(txctx, item.ID)
	}); err != nil {
		return nil, apiError(err)
	}
	return &emptypb.Empty{}, nil
}

func invitationReply(item *model.ProjectInvitation) *adminv1.ProjectInvitationReply {
	return &adminv1.ProjectInvitationReply{
		ProjectId: item.ProjectID,
		Role:      item.Role,
		ExpiresAt: item.ExpiresAt.Format(time.RFC3339),
	}
}

func (s *ProjectInvitationService) validCreator(ctx context.Context, item *model.ProjectInvitation) error {
	member, err := s.members.GetForUpdate(ctx, item.ProjectID, item.CreatedBy)
	if err != nil {
		return permission.ErrForbidden
	}
	if member.Role != "OWNER" && member.Role != "MAINTAINER" {
		return permission.ErrForbidden
	}
	if !permission.CanGrant(permission.ProjectRole(member.Role), permission.ProjectRole(item.Role)) {
		return permission.ErrForbidden
	}
	return nil
}

func (s *ProjectInvitationService) authorizeLocked(ctx context.Context, projectID int64, role string) (int64, error) {
	actor, err := permission.ActorFromContext(ctx)
	if err != nil {
		return 0, err
	}
	err = s.validCreator(ctx, &model.ProjectInvitation{
		ProjectID: projectID,
		CreatedBy: actor.UserID,
		Role:      role,
	})
	return actor.UserID, err
}
