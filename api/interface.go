package api

import (
	"github.com/mdblp/hydrophone/models"
)

type UserRepo interface {
	GetUser(userId, token string) (*models.UserData, error)
	UpdateUser(userId string, user models.UserUpdate, token string) error
}
