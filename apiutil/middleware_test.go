package apiutil

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"

	"github.com/TravisS25/httputil/apiutil/apitest"
	"github.com/TravisS25/httputil/cacheutil/cachetest"

	"github.com/TravisS25/httputil/cacheutil"
	"github.com/pkg/errors"

	"github.com/TravisS25/httputil"
	"github.com/TravisS25/httputil/dbutil/dbtest"
)

func TestAuthMiddleware(t *testing.T) {
	queryUser := "queryUser"
	querySession := "querySession"
	decodeErr := "decodeErr"
	internalErr := "internalErr"
	noRowsErr := "noRowsErr"
	generalErr := "generalErr"
	invalidJSON := "invalidJSON"
	cookieName := "user"

	request := httptest.NewRequest(http.MethodGet, "/url", nil)

	getFuncErr := func(r *http.Request, name string) (*sessions.Session, error) {
		return nil, errors.New("error")
	}
	pingFunc := func() (bool, error) {
		return true, nil
	}
	pingFuncErr := func() (bool, error) {
		return false, errors.New("error")
	}
	mockSessionStore := &cachetest.MockSessionStore{
		GetFunc:  getFuncErr,
		NewFunc:  getFuncErr,
		PingFunc: pingFuncErr,
		SaveFunc: func(r *http.Request, w http.ResponseWriter, s *sessions.Session) error {
			return nil
		},
	}
	getFuncNewSession := func(r *http.Request, name string) (*sessions.Session, error) {
		s := sessions.NewSession(mockSessionStore, cookieName)
		s.IsNew = true
		return s, nil
	}
	getFuncSession := func(r *http.Request, name string) (*sessions.Session, error) {
		s := sessions.NewSession(mockSessionStore, cookieName)
		s.IsNew = false
		return s, nil
	}

	mockHandler := &apitest.MockHandler{
		ServeHTTPFunc: func(w http.ResponseWriter, r *http.Request) {},
	}
	mockDB := &dbtest.MockDB{
		RecoverErrorFunc: func(err error) bool {
			return true
		},
	}

	authConfig := AuthHandlerConfig{
		SessionConfig: cacheutil.SessionConfig{
			SessionName: "user",
			Keys: cacheutil.SessionKeys{
				UserKey: "user",
			},
		},
		QueryForSession: func(w http.ResponseWriter, db httputil.DBInterfaceV2, userID string) (string, error) {
			if userID == "1" {
				return "some session", nil
			}

			if userID == "0" {
				return "", sql.ErrNoRows
			}

			return "", errors.New("error")
		},
	}
	queryDB := func(w http.ResponseWriter, r *http.Request, db httputil.DBInterfaceV2) ([]byte, error) {
		if r.Header.Get(queryUser) == decodeErr {
			return nil, cachetest.NewMockSessionError(nil, "", false, true, false)
		}

		if r.Header.Get(queryUser) == internalErr {
			fmt.Printf("made to internal error\n")
			return nil, cachetest.NewMockSessionError(nil, "", false, false, true)
		}

		if r.Header.Get(queryUser) == noRowsErr {
			return nil, sql.ErrNoRows
		}

		if r.Header.Get(queryUser) == generalErr {
			return nil, errors.New(generalErr)
		}

		if r.Header.Get(queryUser) == invalidJSON {
			sMap := []string{"foobar"}
			return json.Marshal(sMap)
		}

		m := middlewareUser{
			ID:    "1",
			Email: "someemail@email.com",
		}

		if r.Header.Get(querySession) == noRowsErr {
			m.ID = "0"
		}

		if r.Header.Get(querySession) == generalErr {
			m.ID = "-1"
		}

		return json.Marshal(&m)
	}

	authHandler := NewAuthHandler(mockDB, queryDB, authConfig)

	// Testing default settings without cache
	rr := httptest.NewRecorder()
	h := authHandler.MiddlewareFunc(mockHandler)
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Testing with cookie decode error and without cache
	rr = httptest.NewRecorder()
	request.Header.Set(queryUser, decodeErr)
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusBadRequest {
		t.Errorf(statusErrTxt, http.StatusBadRequest, rr.Code)
	}

	// Testing with internal cookie error and without cache
	rr = httptest.NewRecorder()
	request.Header.Set(queryUser, internalErr)
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Testing with sql no rows error and without cache
	rr = httptest.NewRecorder()
	request.Header.Set(queryUser, noRowsErr)
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Testing with server error and without cache
	rr = httptest.NewRecorder()
	request.Header.Set(queryUser, generalErr)
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Testing with invalid json and without cache
	rr = httptest.NewRecorder()
	request.Header.Set(queryUser, invalidJSON)
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up session with session error
	authConfig.SessionStore = mockSessionStore
	authHandler.setConfig(authConfig)

	// Testing session store get error
	rr = httptest.NewRecorder()
	request.Header.Set(queryUser, "")
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up new session with no cookie found
	mockSessionStore.GetFunc = getFuncNewSession
	authConfig.SessionStore = mockSessionStore
	authHandler.setConfig(authConfig)

	// Testing session store get error
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Testing cookie but the session store is still down
	request.AddCookie(sessions.NewCookie("user", "val", &sessions.Options{}))
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Setting up where session backend is back up
	// but grabbing session from database can't be found
	mockSessionStore.PingFunc = pingFunc
	request.Header.Set(querySession, noRowsErr)

	// Testing that query for session returns
	// no row error but with ok status
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Testing that an error occured when trying
	// to query for session and should get internal error
	request.Header.Set(querySession, generalErr)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Testing that we get internal error when trying
	// to get new session from session store
	request.Header.Set(querySession, "")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up for successful new session from session store
	mockSessionStore.NewFunc = getFuncNewSession
	authConfig.SessionStore = mockSessionStore
	authHandler.setConfig(authConfig)

	// Testing successful new session
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Setting up
	mockSessionStore.GetFunc = getFuncSession
	authConfig.SessionStore = mockSessionStore
	authHandler.setConfig(authConfig)

}
