package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	domainauth "rentme/internal/domain/auth"
	domainuser "rentme/internal/domain/user"
)

// UserRepository stores users in memory. Not suitable for production.
type UserRepository struct {
	mu      sync.RWMutex
	byID    map[domainuser.ID]*domainuser.User
	byEmail map[string]domainuser.ID
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		byID:    make(map[domainuser.ID]*domainuser.User),
		byEmail: make(map[string]domainuser.ID),
	}
}

func (r *UserRepository) ByID(ctx context.Context, id domainuser.ID) (*domainuser.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if user, ok := r.byID[id]; ok {
		return cloneUser(user), nil
	}
	return nil, domainuser.ErrNotFound
}

func (r *UserRepository) ByEmail(ctx context.Context, email string) (*domainuser.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byEmail[strings.ToLower(strings.TrimSpace(email))]
	if !ok {
		return nil, domainuser.ErrNotFound
	}
	if user, ok := r.byID[id]; ok {
		return cloneUser(user), nil
	}
	return nil, domainuser.ErrNotFound
}

func (r *UserRepository) Save(ctx context.Context, user *domainuser.User) error {
	if user == nil {
		return domainuser.ErrIDRequired
	}
	id := strings.TrimSpace(string(user.ID))
	if id == "" {
		return domainuser.ErrIDRequired
	}
	emailKey := strings.ToLower(strings.TrimSpace(user.Email))
	if emailKey == "" {
		return domainuser.ErrEmailRequired
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if existingID, ok := r.byEmail[emailKey]; ok && existingID != user.ID {
		return domainuser.ErrEmailAlreadyUsed
	}
	r.byEmail[emailKey] = user.ID
	r.byID[user.ID] = cloneUser(user)
	return nil
}

func cloneUser(u *domainuser.User) *domainuser.User {
	if u == nil {
		return nil
	}
	copyUser := *u
	copyUser.Roles = append([]domainuser.Role(nil), u.Roles...)
	return &copyUser
}

// SessionStore keeps bearer sessions in memory.
type SessionStore struct {
	mu        sync.RWMutex
	tokens    map[domainauth.Token]*domainauth.Session
	userIndex map[domainuser.ID]map[domainauth.Token]struct{}
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		tokens:    make(map[domainauth.Token]*domainauth.Session),
		userIndex: make(map[domainuser.ID]map[domainauth.Token]struct{}),
	}
}

func (s *SessionStore) Save(ctx context.Context, session *domainauth.Session) error {
	if session == nil {
		return domainauth.ErrTokenRequired
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[session.Token] = cloneSession(session)
	if _, ok := s.userIndex[session.UserID]; !ok {
		s.userIndex[session.UserID] = make(map[domainauth.Token]struct{})
	}
	s.userIndex[session.UserID][session.Token] = struct{}{}
	return nil
}

func (s *SessionStore) Get(ctx context.Context, token domainauth.Token) (*domainauth.Session, error) {
	s.mu.RLock()
	session, ok := s.tokens[token]
	s.mu.RUnlock()
	if !ok {
		return nil, domainauth.ErrSessionNotFound
	}
	if session.ExpiresAt.Before(time.Now().UTC()) {
		_ = s.Delete(ctx, token)
		return nil, domainauth.ErrSessionNotFound
	}
	return cloneSession(session), nil
}

func (s *SessionStore) Delete(ctx context.Context, token domainauth.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.tokens[token]
	if !ok {
		return nil
	}
	delete(s.tokens, token)
	if index, ok := s.userIndex[session.UserID]; ok {
		delete(index, token)
		if len(index) == 0 {
			delete(s.userIndex, session.UserID)
		}
	}
	return nil
}

func (s *SessionStore) DeleteByUser(ctx context.Context, userID domainuser.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index, ok := s.userIndex[userID]
	if !ok {
		return nil
	}
	for token := range index {
		delete(s.tokens, token)
	}
	delete(s.userIndex, userID)
	return nil
}

func cloneSession(s *domainauth.Session) *domainauth.Session {
	if s == nil {
		return nil
	}
	copySession := *s
	copySession.Roles = append([]domainuser.Role(nil), s.Roles...)
	return &copySession
}

var _ domainuser.Repository = (*UserRepository)(nil)
var _ domainauth.SessionStore = (*SessionStore)(nil)
