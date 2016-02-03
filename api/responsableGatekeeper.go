package api

import "github.com/tidepool-org/go-common/clients"

type PermissionsResponse struct {
	Permissions clients.Permissions
	Error       error
}

type ResponsableGatekeeper struct {
	UserInGroupResponses    []PermissionsResponse
	SetPermissionsResponses []PermissionsResponse
}

func NewResponsableGatekeeper() *ResponsableGatekeeper {
	return &ResponsableGatekeeper{}
}

func (c *ResponsableGatekeeper) HasResponses() bool {
	return len(c.UserInGroupResponses) > 0 ||
		len(c.SetPermissionsResponses) > 0
}

func (c *ResponsableGatekeeper) Reset() {
	c.UserInGroupResponses = nil
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

func (c *ResponsableGatekeeper) SetPermissions(userID, groupID string, permissions clients.Permissions) (clients.Permissions, error) {
	if len(c.SetPermissionsResponses) > 0 {
		var response PermissionsResponse
		response, c.SetPermissionsResponses = c.SetPermissionsResponses[0], c.SetPermissionsResponses[1:]
		return response.Permissions, response.Error
	}
	panic("SetPermissionsResponses unavailable")
}
