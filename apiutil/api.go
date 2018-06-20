package apiutil

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"bitbucket.org/TravisS25/contractor-tracking/contractor-tracking/contractor-server/config"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
)

var (
	takeLimit         = uint64(100)
	NonSafeOperations = []string{"POST", "PUT", "DELETE"}
)

const (
	VerificationKey  = "%s-verification"
	BodyMessage      = "Must have body"
	InvalidJSON      = "Invalid json"
	InvalidForm      = "Invalid Form"
	ErrServerMessage = "Server error, please try again later"
)

const (
	Insert = "INSERT"
	Update = "UPDATE"
	Delete = "DELETE"
)

func CheckError(err error, message string) {
	if err != nil {
		err = errors.Wrap(err, message)
		fmt.Printf("%+v\n", err)
	}
}

func ServerError(w http.ResponseWriter, err error, customMessage string) {
	w.WriteHeader(http.StatusInternalServerError)

	if customMessage != "" {
		w.Write([]byte(customMessage))
	} else {
		w.Write([]byte(ErrServerMessage))
	}

	CheckError(err, customMessage)
}

func HasFormErrors(w http.ResponseWriter, r *http.Request, err error) bool {
	if err != nil {
		CheckError(err, "")
		w.WriteHeader(http.StatusNotAcceptable)
		payload := err.(validation.Errors)
		SendPayload(w, r, false, map[string]interface{}{
			"errors": payload,
		})
		return true
	}

	return false
}

func SendPayload(w http.ResponseWriter, r *http.Request, addContext bool, payload map[string]interface{}) {
	if addContext {
		if user := GetUser(r); user != nil {
			payload["user"] = user

			if r.Context().Value(GroupCtxKey) != nil {
				payload["groups"] = r.Context().Value(GroupCtxKey).([]string)
			}
		}
	}

	jsonString, err := json.Marshal(payload)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.Write(jsonString)
	}
}

func GetUser(r *http.Request) interface{} {
	if r.Context().Value(UserCtxKey) == nil {
		return nil
	}

	return r.Context().Value(UserCtxKey)
}

func HasBodyError(w http.ResponseWriter, r *http.Request) bool {
	if r.Body == nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(BodyMessage))
		return true
	}

	return false
}

func HasDecodeError(w http.ResponseWriter, err error) bool {
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(InvalidJSON))
		return true
	}

	return false
}

func HasQueryError(
	w http.ResponseWriter,
	err error,
	notFoundMessage string,
	serverErrMessage string,
) bool {
	if err == sql.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(notFoundMessage))
		return true
	} else if err != nil && err != sql.ErrNoRows {
		ServerError(w, err, serverErrMessage)
		return true
	}

	return false
}

func LogoutUser(w http.ResponseWriter, r *http.Request) {
	if r.Context().Value(UserCtxKey) != nil {
		session, _ := config.Store.Get(r, "user")
		session.Options = &sessions.Options{
			MaxAge: -1,
		}
		session.Save(r, w)
	}
}

func GetUserGroups(r *http.Request) []string {
	return r.Context().Value(GroupCtxKey).([]string)
}

func HasGroup(r *http.Request, searchGroups ...string) bool {
	groupArray := r.Context().Value(GroupCtxKey).([]string)

	for _, groupName := range groupArray {
		for _, searchGroup := range searchGroups {
			if searchGroup == groupName {
				return true
			}
		}
	}

	return false
}
