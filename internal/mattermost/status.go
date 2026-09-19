package mattermost

import (
	"context"

	"github.com/mattermost/mattermost/server/public/model"
)

// SetOnline preserves the behavior of the original KMO keep-online helper:
// resolve the current user and explicitly set their Mattermost status to online.
func (s *Service) SetOnline(ctx context.Context) error {
	user, _, err := s.client.GetMe(ctx, "")
	if err != nil {
		return err
	}
	_, _, err = s.client.UpdateUserStatus(ctx, user.Id, &model.Status{UserId: user.Id, Status: model.StatusOnline})
	return err
}
