package api

import "github.com/tidepool-org/go-common/clients"

type PermissionsResponse struct {
	Permissions clients.Permissions
	Error       error
}

type UsersPermissionsResponse struct {
	UsersPermissions clients.UsersPermissions
	Error            error
}

type ResponsableGatekeeper struct {
	UserInGroupResponses    []PermissionsResponse
	UsersInGroupResponses   []UsersPermissionsResponse
	SetPermissionsResponses []PermissionsResponse
}

func NewResponsableGatekeeper() *ResponsableGatekeeper {
	return &ResponsableGatekeeper{}
}

func (c *ResponsableGatekeeper) HasResponses() bool {
	return len(c.UserInGroupResponses) > 0 ||
		len(c.UsersInGroupResponses) > 0 ||
		len(c.SetPermissionsResponses) > 0
}

func (c *ResponsableGatekeeper) Reset() {
	c.UserInGroupResponses = nil
	c.UsersInGroupResponses = nil
	c.SetPermissionsResponses = nil
}

func (c *ResponsableGatekeeper) UserInGroup(userID, groupID string) (clients.Permissions, error) {
	if len(c.UserInGroupResponses) > 0 {
		var response PermissionsResponse
		response, c.UserInGroupResponses = c.UserInGroupResponses[0], c.UserInGroupResponses[1:]
		return response.Permissions, response.Error
	}
	panic("UserInGroupResponses unavailable")
}

func (c *ResponsableGatekeeper) UsersInGroup(groupID string) (clients.UsersPermissions, error) {
	if len(c.UsersInGroupResponses) > 0 {
		var response UsersPermissionsResponse
		response, c.UsersInGroupResponses = c.UsersInGroupResponses[0], c.UsersInGroupResponses[1:]
		return response.UsersPermissions, response.Error
	}
	panic("UsersInGroupResponses unavailable")
}

func (c *ResponsableGatekeeper) SetPermissions(userID, groupID string, permissions clients.Permissions) (clients.Permissions, error) {
	if len(c.SetPermissionsResponses) > 0 {
		var response PermissionsResponse
		response, c.SetPermissionsResponses = c.SetPermissionsResponses[0], c.SetPermissionsResponses[1:]
		return response.Permissions, response.Error
	}
	panic("SetPermissionsResponses unavailable")
}
