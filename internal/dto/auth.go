package dto

import "github.com/imlargo/medusa/internal/models"

type LoginWithPassword struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type AuthResponse struct {
	User   models.User `json:"user"`
	Tokens AuthTokens  `json:"tokens"`
}

type AuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
