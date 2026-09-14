package auth

import (
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"watcher/internal/config"
	"watcher/internal/store"
)

type Service struct {
	store  *store.Store
	secret []byte
	days   int
}

func New(st *store.Store, cfg config.Config) *Service {
	days := cfg.JWTDays
	if days <= 0 {
		days = 7
	}
	return &Service{store: st, secret: []byte(cfg.JWTSecret), days: days}
}

func (s *Service) Bootstrap(user, password string) error {
	n, err := s.store.UserCount()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.store.InsertUser(user, hash)
	return err
}

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (s *Service) Login(name, password string) (token string, user *store.User, err error) {
	u, err := s.store.GetUserByName(name)
	if err != nil || u == nil {
		return "", nil, err
	}
	if !CheckPassword(u.PasswordHash, password) {
		return "", nil, nil
	}
	tok, err := s.Sign(u.ID)
	return tok, u, err
}

func (s *Service) Sign(userID int64) (string, error) {
	claims := jwt.MapClaims{
		"sub": strconv.FormatInt(userID, 10),
		"exp": time.Now().Add(time.Duration(s.days) * 24 * time.Hour).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(s.secret)
}

func (s *Service) Parse(token string) (int64, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, jwt.ErrSignatureInvalid
		}
		return s.secret, nil
	})
	if err != nil || !parsed.Valid {
		return 0, err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return 0, jwt.ErrTokenInvalidClaims
	}
	sub, _ := claims["sub"].(string)
	id, err := strconv.ParseInt(sub, 10, 64)
	return id, err
}

func (s *Service) UpdatePassword(id int64, old, neu string) error {
	u, err := s.store.GetUserByID(id)
	if err != nil || u == nil {
		return err
	}
	if !CheckPassword(u.PasswordHash, old) {
		return errWrongPassword
	}
	hash, err := HashPassword(neu)
	if err != nil {
		return err
	}
	return s.store.UpdatePassword(id, hash)
}

type passError string

func (e passError) Error() string { return string(e) }

const errWrongPassword passError = "旧密码不正确"

func IsWrongPassword(err error) bool { return err == errWrongPassword }
