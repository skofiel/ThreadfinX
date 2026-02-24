package src

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"threadfin/src/internal/authentication"
)

func activatedSystemAuthentication() (err error) {

	err = authentication.Init(System.Folder.Config, 60)
	if err != nil {
		return
	}

	var defaults = make(map[string]interface{})
	defaults["authentication.web"] = false
	defaults["authentication.pms"] = false
	defaults["authentication.xml"] = false
	defaults["authentication.api"] = false
	err = authentication.SetDefaultUserData(defaults)

	return
}

func createFirstUserForAuthentication(username, password string) (token string, err error) {

	err = authentication.CreateDefaultUser(username, password)
	if err != nil {
		ShowError(err, 0)
		return
	}

	token, err = authentication.UserAuthentication(username, password)
	if err != nil {
		ShowError(err, 0)
		return
	}

	token, err = authentication.CheckTheValidityOfTheToken(token)
	if err != nil {
		ShowError(err, 0)
		return
	}

	var userData = make(map[string]interface{})
	userData["username"] = username
	userData["authentication.web"] = true
	userData["authentication.pms"] = true
	userData["authentication.m3u"] = true
	userData["authentication.xml"] = true
	userData["authentication.api"] = false
	userData["defaultUser"] = true

	userID, err := authentication.GetUserID(token)
	if err != nil {
		ShowError(err, 0)
		return
	}

	err = authentication.WriteUserData(userID, userData)
	if err != nil {
		ShowError(err, 0)
		return
	}

	return
}

func tokenAuthentication(token string) (newToken string, err error) {

	if System.ConfigurationWizard {
		return
	}

	newToken, err = authentication.CheckTheValidityOfTheToken(token)

	return
}

func basicAuth(r *http.Request, level string) (username string, err error) {

	err = errors.New("User authentication failed")

	auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)

	if len(auth) != 2 || auth[0] != "Basic" {
		return
	}

	payload, decodeErr := base64.StdEncoding.DecodeString(auth[1])
	if decodeErr != nil {
		return
	}
	pair := strings.SplitN(string(payload), ":", 2)
	if len(pair) != 2 {
		return
	}

	username = pair[0]
	var password = pair[1]

	token, err := authentication.UserAuthentication(username, password)

	if err != nil {
		return
	}

	err = checkAuthorizationLevel(token, level)

	return
}

// urlAuth supports both URL query parameters and HTTP Basic Authentication.
// Basic Auth is preferred when available.
func urlAuth(r *http.Request, requestType string) (err error) {
	var level, token string
	var username, password string

	// Prefer Basic Auth header over query parameters
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 && parts[0] == "Basic" {
			if payload, decErr := base64.StdEncoding.DecodeString(parts[1]); decErr == nil {
				if pair := strings.SplitN(string(payload), ":", 2); len(pair) == 2 {
					username = pair[0]
					password = pair[1]
				}
			}
		}
	}

	// Fall back to query parameters for backwards compatibility
	if username == "" {
		username = r.URL.Query().Get("username")
		password = r.URL.Query().Get("password")
	}

	switch requestType {

	case "m3u":
		level = "authentication.m3u"
		if Settings.AuthenticationM3U {
			token, err = authentication.UserAuthentication(username, password)
			if err != nil {
				return
			}
			err = checkAuthorizationLevel(token, level)
		}

	case "xml":
		level = "authentication.xml"
		if Settings.AuthenticationXML {
			token, err = authentication.UserAuthentication(username, password)
			if err != nil {
				return
			}
			err = checkAuthorizationLevel(token, level)
		}

	}

	return
}

func checkAuthorizationLevel(token, level string) (err error) {

	userID, err := authentication.GetUserID(token)
	if err != nil {
		return
	}

	userData, err := authentication.ReadUserData(userID)
	if err != nil {
		return
	}

	if len(userData) > 0 {

		if v, ok := userData[level].(bool); ok {

			if !v {
				err = errors.New("No authorization")
			}

		} else {
			userData[level] = false
			if writeErr := authentication.WriteUserData(userID, userData); writeErr != nil {
				ShowError(writeErr, 0)
			}
			err = errors.New("No authorization")
		}

	} else {
		if writeErr := authentication.WriteUserData(userID, userData); writeErr != nil {
			ShowError(writeErr, 0)
		}
		err = errors.New("No authorization")
	}

	return
}
