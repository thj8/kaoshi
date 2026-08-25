package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims 管理员与答题用户共用 token 结构
type Claims struct {
	Role    string  `json:"role"` // admin / user
	UserID  int64   `json:"uid"`  // 用户ID（admin=0）
	Nick    string  `json:"nick"` // 昵称
	QuizID  int64   `json:"qid"`  // 用户加入的活动ID（admin=0）
	QuizIDs []int64 `json:"qids"` // admin 可管理的活动（预留，暂不校验）
	jwt.RegisteredClaims
}

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

var (
	ErrTokenInvalid = errors.New("token invalid")
	secret          []byte
	ttl             = 24 * time.Hour
)

func Init(s string, tokenTTL time.Duration) {
	secret = []byte(s)
	if tokenTTL > 0 {
		ttl = tokenTTL
	}
}

func Sign(claims *Claims) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func Parse(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrTokenInvalid
		}
		return secret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrTokenInvalid
	}
	c, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrTokenInvalid
	}
	return c, nil
}
