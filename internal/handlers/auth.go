package handlers

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/internal/dto"
	_ "github.com/imlargo/medusa/internal/models"
	"github.com/imlargo/medusa/internal/services"
	"github.com/imlargo/medusa/pkg/medusa"
	"github.com/imlargo/medusa/pkg/medusa/core/handler"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

type AuthHandler struct {
	*handler.Handler
	authService services.AuthService
}

func NewAuthHandler(handler *handler.Handler, authService services.AuthService) *AuthHandler {
	return &AuthHandler{
		Handler:     handler,
		authService: authService,
	}
}

// @Summary		Login user
// @Router			/v1/auth/login [post]
// @Description	Login user with email and password
// @Tags		auth
// @Accept		json
// @Param		payload	body	dto.LoginWithPassword	true	"Login user request payload"
// @Produce		json
// @Success		200	{object}	dto.AuthResponse	"User logged in successfully"
// @Failure		400	{object}	responses.ErrorResponse	"Bad Request"
// @Failure		500	{object}	responses.ErrorResponse	"Internal Server Error"
// @Security     BearerAuth
func (a *AuthHandler) LoginWithPassword(c *gin.Context) {
	var payload dto.LoginWithPassword
	if err := c.ShouldBindJSON(&payload); err != nil {
		responses.ErrorValidation(c, err)
		return
	}

	authResponse, err := a.authService.LoginWithPassword(context.Background(), payload.Email, payload.Password)
	if err != nil {
		responses.ErrorInternalServerWithMessage(c, err.Error(), nil)
		return
	}

	responses.SuccessOK(c, authResponse)
}

// @Summary		Register user
// @Router			/v1/auth/register [post]
// @Description	Register a new user with email, password
// @Tags		auth
// @Accept		json
// @Param		payload	body	dto.RegisterUser	true	"Register user request payload"
// @Produce		json
// @Success		200	{object}	dto.AuthResponse	"User registered successfully
// @Failure		400	{object}	responses.ErrorResponse	"Bad Request"
// @Failure		500	{object}	responses.ErrorResponse	"Internal Server Error
// @Security     BearerAuth
func (a *AuthHandler) Register(c *gin.Context) {
	var payload dto.RegisterUser
	if err := c.ShouldBindJSON(&payload); err != nil {
		responses.ErrorValidation(c, err)
		return
	}

	authData, err := a.authService.RegisterWithPassword(context.Background(), &payload)
	if err != nil {
		responses.ErrorInternalServerWithMessage(c, err.Error(), nil)
		return
	}

	responses.SuccessOK(c, authData)
}

// @Summary		Get user info
// @Router			/v1/auth/user [get]
// @Description	Get the authenticated user's information
// @Tags		auth
// @Produce		json
// @Success		200	{object}	models.User	"Authenticated user's
// @Failure		401	{object}	responses.ErrorResponse	"Unauthorized"
// @Failure		500	{object}	responses.ErrorResponse	"Internal Server Error
// @Security     BearerAuth
func (a *AuthHandler) GetUser(c *gin.Context) {

	userID, exists := medusa.GetUserID(c)
	if !exists {
		responses.ErrorUnauthorized(c, "User not authenticated")
		return
	}

	user, err := a.authService.GetUser(context.Background(), userID)
	if err != nil {
		responses.ErrorInternalServerWithMessage(c, err.Error(), nil)
		return
	}

	responses.SuccessOK(c, user)
}
