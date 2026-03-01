package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type AuthProfile struct {
	Name        string            `json:"name"`
	Provider    string            `json:"provider"`
	Credentials map[string]string `json:"credentials"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type AuthService struct {
	mu            sync.RWMutex
	profiles      map[string]*AuthProfile
	activeProfile string
	configDir     string
}

func NewAuthService(configDir string) *AuthService {
	return &AuthService{
		profiles:      make(map[string]*AuthProfile),
		configDir:     configDir,
		activeProfile: "default",
	}
}

func (s *AuthService) LoadProfiles() error {
	profilesFile := filepath.Join(s.configDir, "auth-profiles.json")

	data, err := os.ReadFile(profilesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read auth profiles: %w", err)
	}

	var profiles map[string]*AuthProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return fmt.Errorf("failed to parse auth profiles: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.profiles = profiles

	return nil
}

func (s *AuthService) SaveProfiles() error {
	s.mu.RLock()
	profiles := s.profiles
	s.mu.RUnlock()

	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal auth profiles: %w", err)
	}

	profilesFile := filepath.Join(s.configDir, "auth-profiles.json")
	if err := os.WriteFile(profilesFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write auth profiles: %w", err)
	}

	return nil
}

func (s *AuthService) AddProfile(name string, provider string, credentials map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile := &AuthProfile{
		Name:        name,
		Provider:    provider,
		Credentials: credentials,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	s.profiles[name] = profile
	return s.SaveProfiles()
}

func (s *AuthService) GetProfile(name string) (*AuthProfile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, exists := s.profiles[name]
	return profile, exists
}

func (s *AuthService) GetActiveProfile() (*AuthProfile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, exists := s.profiles[s.activeProfile]
	return profile, exists
}

func (s *AuthService) SetActiveProfile(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.profiles[name]; !exists {
		return fmt.Errorf("profile not found: %s", name)
	}

	s.activeProfile = name
	return nil
}

func (s *AuthService) RemoveProfile(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.profiles[name]; !exists {
		return fmt.Errorf("profile not found: %s", name)
	}

	delete(s.profiles, name)

	if s.activeProfile == name {
		s.activeProfile = "default"
	}

	return s.SaveProfiles()
}

func (s *AuthService) UpdateCredentials(name string, credentials map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	profile, exists := s.profiles[name]
	if !exists {
		return fmt.Errorf("profile not found: %s", name)
	}

	profile.Credentials = credentials
	profile.UpdatedAt = time.Now()

	return s.SaveProfiles()
}

func (s *AuthService) IsExpired(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profile, exists := s.profiles[name]
	if !exists || profile.ExpiresAt == nil {
		return false
	}

	return time.Now().After(*profile.ExpiresAt)
}

func (s *AuthService) ListProfiles() []*AuthProfile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profiles := make([]*AuthProfile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		profiles = append(profiles, profile)
	}

	return profiles
}

type TokenStore struct {
	mu        sync.RWMutex
	tokens    map[string]*Token
	configDir string
}

type Token struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token,omitempty"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	TokenType    string     `json:"token_type"`
	Scope        string     `json:"scope,omitempty"`
}

func NewTokenStore(configDir string) *TokenStore {
	return &TokenStore{
		tokens:    make(map[string]*Token),
		configDir: configDir,
	}
}

func (s *TokenStore) Load() error {
	tokenFile := filepath.Join(s.configDir, "tokens.json")

	data, err := os.ReadFile(tokenFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read tokens: %w", err)
	}

	var tokens map[string]*Token
	if err := json.Unmarshal(data, &tokens); err != nil {
		return fmt.Errorf("failed to parse tokens: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens = tokens

	return nil
}

func (s *TokenStore) Save() error {
	s.mu.RLock()
	tokens := s.tokens
	s.mu.RUnlock()

	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	tokenFile := filepath.Join(s.configDir, "tokens.json")
	if err := os.WriteFile(tokenFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write tokens: %w", err)
	}

	return nil
}

func (s *TokenStore) Set(key string, token *Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[key] = token
	return s.Save()
}

func (s *TokenStore) Get(key string) (*Token, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, exists := s.tokens[key]
	return token, exists
}

func (s *TokenStore) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tokens, key)
	return s.Save()
}

func (s *TokenStore) IsExpired(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	token, exists := s.tokens[key]
	if !exists || token.ExpiresAt == nil {
		return false
	}

	return time.Now().After(*token.ExpiresAt)
}
