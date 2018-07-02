package apiutil

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
)

var (
	// NonSafeOperations is slice of http methods that are not safe
	NonSafeOperations = []string{"POST", "PUT", "DELETE"}

	// ErrBodyMessage is used for when a post/put request does not contain a body in request
	ErrBodyMessage = errors.New("Must have body")

	// ErrInvalidJSON is used when there is an error unmarshalling a struct
	ErrInvalidJSON = errors.New("Invalid json")

	// ErrServerMessage is used when there is a server error
	ErrServerMessage = errors.New("Server error, please try again later")
)

// LogError will take given error and append to log file given
func LogError(err error, customMessage string, logFile string) error {
	err = errors.Wrap(err, customMessage)
	file, err := os.Open(logFile)

	if err != nil {
		return err
	}

	defer file.Close()

	if _, err = file.WriteString(err.Error()); err != nil {
		return err
	}

	return nil
}

// CheckError simply prints given error in verbose to stdout
func CheckError(err error, customMessage string) {
	err = errors.Wrap(err, customMessage)
	fmt.Printf("%+v\n", err)
}

// ServerError takes given err along with customMessage and writes back to client
// then logs the error given the logFile
func ServerError(w http.ResponseWriter, err error, customMessage string, logFile string) {
	w.WriteHeader(http.StatusInternalServerError)

	if customMessage != "" {
		w.Write([]byte(customMessage))
	} else {
		w.Write([]byte(ErrServerMessage.Error()))
	}

	CheckError(err, customMessage)
	LogError(err, customMessage, logFile)
}

// HasFormErrors determines if err is nil and if it is, convert it to json form
// with which form fields have errors and send to client with 406 error
// If err is not nil, returns true else false
func HasFormErrors(w http.ResponseWriter, r *http.Request, err error) bool {
	if err != nil {
		CheckError(err, "")
		payload := err.(validation.Errors)
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(payload.Error()))
		// SendPayload(w, r, false, map[string]interface{}{
		// 	"errors": payload,
		// })
		return true
	}

	return false
}

// SendPayload is a wrapper for converting the payload map parameter into json and sending to the client
// If addUserContext parameter is set to true, the json sent back will also include user and groups ctx
func SendPayload(w http.ResponseWriter, r *http.Request, addUserContext bool, payload map[string]interface{}) {
	if addUserContext {
		if user := GetUser(r); user != nil {
			payload["user"] = user

			if r.Context().Value(groupCtxKey) != nil {
				payload["groups"] = r.Context().Value(groupCtxKey).([]string)
			}
		}
	}

	jsonString, err := json.Marshal(payload)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(ErrInvalidJSON.Error()))
	} else {
		w.Write(jsonString)
	}
}

// GetUser returns a user if set in userctx, else returns nil
func GetUser(r *http.Request) interface{} {
	if r.Context().Value(userCtxKey) == nil {
		return nil
	}

	return r.Context().Value(userCtxKey)
}

// HasBodyError checks if the "Body" field of the request parameter is nil or not
// If nil, we write to client with error message, 406 status and return true
// Else return false
func HasBodyError(w http.ResponseWriter, r *http.Request) bool {
	if r.Body == nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(ErrBodyMessage.Error()))
		return true
	}

	return false
}

// HasDecodeError is a wrapper for json decode err
// The passed error should come from trying to decode json
// If the err is not nil, write to client with error message, 406 status and return true
// Else return false
func HasDecodeError(w http.ResponseWriter, err error) bool {
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(ErrInvalidJSON.Error()))
		return true
	}

	return false
}

// HasQueryError is wrapper for determining if err equals "sql.ErrNoRows"
// If it does, we write to client with not found message, 404 status and return true
// Else return false
func HasQueryError(w http.ResponseWriter, err error, notFoundMessage string) bool {
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(notFoundMessage))
		return true
	}

	return false
}

// LogoutUser deletes user session based on session object passed along with userSession parameter
// If userSession is empty string, then string "user" will be used to delete from session object
func LogoutUser(w http.ResponseWriter, r *http.Request, sessionStore sessions.Store, userSession string) error {
	if r.Context().Value(userCtxKey) != nil {
		var session *sessions.Session
		var err error

		if userSession == "" {
			session, err = sessionStore.Get(r, "user")
		} else {
			session, err = sessionStore.Get(r, userSession)
		}

		if err != nil {
			return err
		}

		session.Options = &sessions.Options{
			MaxAge: -1,
		}
		session.Save(r, w)
	}

	return nil
}

// GetUserGroups is wrapper for to returning group string slice from context of request
// If there is no groupctx, returns nil
func GetUserGroups(r *http.Request) []string {
	if r.Context().Value(groupCtxKey) != nil {
		return r.Context().Value(groupCtxKey).([]string)
	}

	return nil
}

// HasGroup is a wrapper for finding if given groups names is in
// group context of given request
// If a group name is found, return true; else returns false
// The search is based on OR logic so if any one of the given strings
// is found, function will return true
func HasGroup(r *http.Request, searchGroups ...string) bool {
	groupArray := r.Context().Value(groupCtxKey).([]string)

	for _, groupName := range groupArray {
		for _, searchGroup := range searchGroups {
			if searchGroup == groupName {
				return true
			}
		}
	}

	return false
}
