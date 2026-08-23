package services

import (
	"context"
	"errors"
	"strings"

	"github.com/imlargo/medusa/internal/dto"
	"github.com/imlargo/medusa/internal/models"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserService interface {
	CreateUser(ctx context.Context, user *dto.RegisterUser) (*models.User, error)
	DeleteUser(ctx context.Context, userID uint) error
	GetUserByID(ctx context.Context, userID uint) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
}

type userServiceImpl struct {
	*Service
}

func NewUserService(container *Service) UserService {
	return &userServiceImpl{
		Service: container,
	}
}

// CreateUser registers a new account with a hashed password.
func (s *userServiceImpl) CreateUser(ctx context.Context, registerUser *dto.RegisterUser) (*models.User, error) {
	email := strings.ToLower(strings.TrimSpace(registerUser.Email))

	// Distinguish "no such user" from "the lookup failed". Discarding this error
	// meant a database outage read as a free email address, so registration
	// carried on and failed later with something unrelated.
	existing, err := s.store.Users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, responses.Internal(err)
	}
	if existing != nil {
		return nil, responses.Conflict("a user with this email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(registerUser.Password), bcrypt.DefaultCost)
	if err != nil {
		// The cause is kept for the log; the client gets the generic message.
		return nil, responses.InternalWithMessage("could not process the password", err)
	}

	user := &models.User{
		Email:    email,
		Password: string(hashedPassword),
	}

	if err := s.store.Users.Create(ctx, user); err != nil {
		return nil, responses.Internal(err)
	}

	return user, nil
}

// DeleteUser removes an account.
func (s *userServiceImpl) DeleteUser(ctx context.Context, userID uint) error {
	if err := s.store.Users.Delete(ctx, userID); err != nil {
		return classifyUserError(err)
	}

	return nil
}

// GetUserByID looks an account up by its primary key.
func (s *userServiceImpl) GetUserByID(ctx context.Context, userID uint) (*models.User, error) {
	user, err := s.store.Users.Get(ctx, userID)
	if err != nil {
		return nil, classifyUserError(err)
	}

	return user, nil
}

// GetUserByEmail looks an account up by email. The lookup is case-insensitive.
func (s *userServiceImpl) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user, err := s.store.Users.GetByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, classifyUserError(err)
	}

	return user, nil
}

// classifyUserError turns a repository error into one carrying an HTTP status,
// so a missing row answers 404 instead of the 500 a raw gorm error produces.
func classifyUserError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return responses.NotFound("user")
	}

	return responses.Internal(err)
}
