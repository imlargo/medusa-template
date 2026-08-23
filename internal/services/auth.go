package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/imlargo/medusa/internal/dto"
	"github.com/imlargo/medusa/internal/models"
	"github.com/imlargo/medusa/pkg/medusa/core/jwt"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService interface {
	GetUser(ctx context.Context, userID uint) (*models.User, error)
	LoginWithPassword(ctx context.Context, email, password string) (*dto.AuthResponse, error)
	RegisterWithPassword(ctx context.Context, user *dto.RegisterUser) (*dto.AuthResponse, error)
}

type authService struct {
	*Service
	jwtAuth     *jwt.JWT
	userService UserService
}

func NewAuthService(container *Service, userSrv UserService, jwtAuth *jwt.JWT) AuthService {
	return &authService{
		Service:     container,
		userService: userSrv,
		jwtAuth:     jwtAuth,
	}
}

func (s *authService) GetUser(ctx context.Context, userID uint) (*models.User, error) {
	user, err := s.store.Users.Get(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, responses.NotFound("user")
	}
	if err != nil {
		return nil, responses.Internal(err)
	}

	return user, nil
}

func (s *authService) LoginWithPassword(ctx context.Context, email, password string) (*dto.AuthResponse, error) {
	user, err := s.store.Users.GetByEmail(ctx, strings.ToLower(email))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, responses.Internal(err)
	}

	// A missing user and a wrong password return the same error on purpose: a
	// distinguishable response turns this endpoint into an account enumerator.
	if user == nil {
		return nil, errInvalidCredentials()
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errInvalidCredentials()
	}

	accessExpiration := time.Now().Add(s.config.Auth.TokenExpiration)
	refreshExpiration := time.Now().Add(s.config.Auth.RefreshExpiration)
	accessToken, err := s.jwtAuth.GenerateToken(user.ID, accessExpiration)
	if err != nil {
		return nil, responses.InternalWithMessage("could not issue the access token", err)
	}

	refreshToken, err := s.jwtAuth.GenerateToken(user.ID, refreshExpiration)
	if err != nil {
		return nil, responses.InternalWithMessage("could not issue the refresh token", err)
	}

	authResponse := &dto.AuthResponse{
		User: *user,
		Tokens: dto.AuthTokens{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		},
	}

	return authResponse, nil
}

func (s *authService) RegisterWithPassword(ctx context.Context, user *dto.RegisterUser) (*dto.AuthResponse, error) {

	createdUser, err := s.userService.CreateUser(ctx, user)
	if err != nil {
		return nil, err
	}

	accessExpiration := time.Now().Add(s.config.Auth.TokenExpiration)
	refreshExpiration := time.Now().Add(s.config.Auth.RefreshExpiration)
	accessToken, err := s.jwtAuth.GenerateToken(createdUser.ID, accessExpiration)
	if err != nil {
		return nil, responses.InternalWithMessage("could not issue the access token", err)
	}

	refreshToken, err := s.jwtAuth.GenerateToken(createdUser.ID, refreshExpiration)
	if err != nil {
		return nil, responses.InternalWithMessage("could not issue the refresh token", err)
	}

	authResponse := &dto.AuthResponse{
		User: *createdUser,
		Tokens: dto.AuthTokens{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		},
	}

	return authResponse, nil
}

// errInvalidCredentials is the single response for any failed password login,
// so callers cannot tell a wrong password from an unknown account.
func errInvalidCredentials() error {
	return responses.Unauthorized("invalid email or password")
}
