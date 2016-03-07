package contacts

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
)

type AuthDetails struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
}

// AuthManager allows to get access token and renew access token.
type AuthManager interface {
	AccessToken() (string, error)
	Renew() (string, error)
}

type AccessTokenRetriever interface {
	Retrieve(refreshToken string) (string, error)
}

type AuthStorage interface {
	Load() (*AuthDetails, error)
	Save(authDetails *AuthDetails) error
}

type FileAuthStorage struct {
	Path string
}

func (s *FileAuthStorage) Load() (*AuthDetails, error) {
	file, err := os.Open(s.Path)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}
	authDetails := &AuthDetails{}
	err = json.Unmarshal(data, authDetails)
	if err != nil {
		return nil, err
	}
	return authDetails, nil
}

func (s *FileAuthStorage) Save(authDetails *AuthDetails) error {
	data, err := json.Marshal(authDetails)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(s.Path, data, 0644)
}

type StandardAuthManager struct {
	AccessTokenRetriever AccessTokenRetriever
	AuthStorage          AuthStorage
}

type StandardAccessTokenRetriever struct {
	ClientID     string
	GoogleSecret string
}

func (r *StandardAccessTokenRetriever) Retrieve(refreshToken string) (string, error) {
	requestParams := url.Values{
		"refresh_token": []string{refreshToken},
		"grant_type":    []string{"refresh_token"},
		"client_id":     []string{r.ClientID},
		"client_secret": []string{r.GoogleSecret},
	}

	resp, err := http.PostForm("https://www.googleapis.com/oauth2/v4/token", requestParams)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return unmarshalAccessToken(buf.Bytes())
}

func unmarshalAccessToken(buf []byte) (string, error) {
	authDetails := &AuthDetails{}
	err := json.Unmarshal(buf, authDetails)

	return authDetails.AccessToken, err
}

func (m *StandardAuthManager) AccessToken() (string, error) {
	authDetails, err := m.AuthStorage.Load()
	if err != nil {
		return "", err
	}
	if authDetails.AccessToken != "" {
		return authDetails.AccessToken, nil
	}
	return m.exchangeRefreshTokenForAccessTokenAndStore(authDetails)
}

func (m *StandardAuthManager) Renew() (string, error) {
	authDetails, err := m.AuthStorage.Load()
	if err != nil {
		return "", err
	}
	authDetails.AccessToken = ""
	return m.exchangeRefreshTokenForAccessTokenAndStore(authDetails)
}

func (m *StandardAuthManager) exchangeRefreshTokenForAccessTokenAndStore(authDetails *AuthDetails) (string, error) {
	if authDetails.RefreshToken == "" {
		return "", errors.New("no refresh token provided")
	}

	accessToken, err := m.AccessTokenRetriever.Retrieve(authDetails.RefreshToken)
	if err != nil {
		return "", err
	}
	authDetails.AccessToken = accessToken

	// attempt to persist authDetails
	_ = m.AuthStorage.Save(authDetails)
	// TODO: add warning that accessToken couldn't be persisted
	return accessToken, nil
}
