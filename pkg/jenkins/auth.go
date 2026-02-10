package jenkins

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// AuthenticateAndGenerateToken authenticates with Jenkins Basic Auth and generates a new API Token.
// It returns the new token value or an error.
func AuthenticateAndGenerateToken(jenkinsURL, username, password string) (string, error) {
	// Setup Client with CookieJar
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
	}

	// 1. Get Crumb
	crumbURL := fmt.Sprintf("%s/crumbIssuer/api/xml?xpath=concat(//crumbRequestField,\":\",//crumb)", jenkinsURL)
	req, err := http.NewRequest("GET", crumbURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating crumb request: %w", err)
	}
	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error fetching crumb: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return "", fmt.Errorf("authentication failed: check your username and password (status: %s)", resp.Status)
		}
		return "", fmt.Errorf("failed to get crumb (status: %s)", resp.Status)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	crumbAndField := string(bodyBytes)
	// Format: CrumbField:CrumbValue
	parts := strings.SplitN(crumbAndField, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid crumb response format")
	}
	crumbHeader := parts[0]
	crumbValue := parts[1]

	// 2. Generate API Token
	tokenName := fmt.Sprintf("jw-cli-%d", time.Now().Unix())
	generateURL := fmt.Sprintf("%s/me/descriptorByName/jenkins.security.ApiTokenProperty/generateNewToken", jenkinsURL)

	data := url.Values{}
	data.Set("newTokenName", tokenName)

	req, err = http.NewRequest("POST", generateURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("error creating token generation request: %w", err)
	}
	req.SetBasicAuth(username, password)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(crumbHeader, crumbValue)

	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error generating token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to generate token (status: %s)", resp.Status)
	}

	type TokenResponse struct {
		Status string `json:"status"`
		Data   struct {
			TokenName  string `json:"tokenName"`
			TokenValue string `json:"tokenValue"`
		} `json:"data"`
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("error parsing token response: %w", err)
	}

	if tokenResp.Status != "ok" {
		return "", fmt.Errorf("error from Jenkins: status not ok")
	}

	return tokenResp.Data.TokenValue, nil
}


