package handlers

import (
	"github.com/imlargo/medusa/internal/dto"
	"github.com/imlargo/medusa/internal/models"
	"github.com/imlargo/medusa/internal/services"
	"github.com/imlargo/medusa/pkg/medusa"
	"github.com/imlargo/medusa/pkg/medusa/core/handler"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
)

// AuthHandler serves the authentication endpoints.
//
// Every method returns its result and its error. Binding, validation, status
// selection and response rendering are handled by the medusa.Handle adapters the
// routes wrap these in, so nothing here writes to the response.
type AuthHandler struct {
	*handler.Handler

	authService services.AuthService
}

func NewAuthHandler(base *handler.Handler, authService services.AuthService) *AuthHandler {
	return &AuthHandler{
		Handler:     base,
		authService: authService,
	}
}

// LoginWithPassword exchanges credentials for a token pair.
//
//	@Summary		Login user
//	@Router			/v1/auth/login [post]
//	@Description	Login with email and password
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		dto.LoginWithPassword	true	"Credentials"
//	@Success		200		{object}	responses.SuccessResponse{data=dto.AuthResponse}	"Logged in"
//	@Failure		400		{object}	responses.ErrorResponse	"Bad Request"
//	@Failure		401		{object}	responses.ErrorResponse	"Invalid credentials"
//	@Failure		500		{object}	responses.ErrorResponse	"Internal Server Error"
func (a *AuthHandler) LoginWithPassword(ctx *medusa.Context, in *dto.LoginWithPassword) (*dto.AuthResponse, error) {
	return a.authService.LoginWithPassword(ctx.Ctx(), in.Email, in.Password)
}

// Register creates an account and returns a token pair for it.
//
//	@Summary		Register user
//	@Router			/v1/auth/register [post]
//	@Description	Register a new user with email and password
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		dto.RegisterUser	true	"Registration payload"
//	@Success		201		{object}	responses.SuccessResponse{data=dto.AuthResponse}	"Registered"
//	@Failure		400		{object}	responses.ErrorResponse	"Bad Request"
//	@Failure		409		{object}	responses.ErrorResponse	"Email already registered"
//	@Failure		500		{object}	responses.ErrorResponse	"Internal Server Error"
func (a *AuthHandler) Register(ctx *medusa.Context, in *dto.RegisterUser) (*dto.AuthResponse, error) {
	return a.authService.RegisterWithPassword(ctx.Ctx(), in)
}

// GetUser returns the authenticated user.
//
//	@Summary		Get user info
//	@Router			/v1/auth/user [get]
//	@Description	Get the authenticated user's information
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	responses.SuccessResponse{data=models.User}	"The authenticated user"
//	@Failure		401	{object}	responses.ErrorResponse	"Unauthorized"
//	@Failure		500	{object}	responses.ErrorResponse	"Internal Server Error"
//	@Security		BearerAuth
func (a *AuthHandler) GetUser(ctx *medusa.Context) (*models.User, error) {
	userID, ok := ctx.UserID()
	if !ok {
		// Only reachable if the route is registered without the auth middleware.
		return nil, responses.Unauthorized("authentication required")
	}

	return a.authService.GetUser(ctx.Ctx(), userID)
}
