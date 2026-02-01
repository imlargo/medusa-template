package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/imlargo/medusa/internal/dto"
	"github.com/imlargo/medusa/internal/models"
	"github.com/imlargo/medusa/pkg/medusa/core/jwt"
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
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *authService) LoginWithPassword(ctx context.Context, email, password string) (*dto.AuthResponse, error) {
	user, err := s.store.Users.GetByEmail(ctx, strings.ToLower(email))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if user == nil {
		return nil, errors.New("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errors.New("invalid email or password")
	}

	accessExpiration := time.Now().Add(s.config.Auth.TokenExpiration)
	refreshExpiration := time.Now().Add(s.config.Auth.RefreshExpiration)
	accessToken, err := s.jwtAuth.GenerateToken(user.ID, accessExpiration)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtAuth.GenerateToken(user.ID, refreshExpiration)
	if err != nil {
		return nil, err
	}

	AuthTokens := &dto.AuthResponse{
		User: *user,
		Tokens: dto.AuthTokens{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		},
	}

	return AuthTokens, nil
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
		return nil, err
	}

	refreshToken, err := s.jwtAuth.GenerateToken(createdUser.ID, refreshExpiration)
	if err != nil {
		return nil, err
	}

	AuthTokens := &dto.AuthResponse{
		User: *createdUser,
		Tokens: dto.AuthTokens{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		},
	}

	return AuthTokens, nil
}
