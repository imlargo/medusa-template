package services

import (
	"context"
	"errors"
	"strings"

	"github.com/imlargo/medusa/internal/dto"
	"github.com/imlargo/medusa/internal/models"
	"golang.org/x/crypto/bcrypt"
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

func (s *userServiceImpl) CreateUser(ctx context.Context, registerUser *dto.RegisterUser) (*models.User, error) {

	registerUser.Email = strings.ToLower(registerUser.Email)

	// Validate user data
	user := &models.User{
		Email:    registerUser.Email,
		Password: registerUser.Password,
	}

	existingUser, _ := s.store.Users.GetByEmail(ctx, user.Email)
	if existingUser != nil {
		return nil, errors.New("user with this email already exists")
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}
	user.Password = string(hashedPassword)

	if err := s.store.Users.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userServiceImpl) DeleteUser(ctx context.Context, userID uint) error {
	return s.store.Users.Delete(ctx, userID)
}

func (s *userServiceImpl) GetUserByID(ctx context.Context, userID uint) (*models.User, error) {
	return s.store.Users.Get(ctx, userID)
}

func (s *userServiceImpl) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.store.Users.GetByEmail(ctx, email)
}
