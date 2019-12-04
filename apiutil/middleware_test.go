package apiutil

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/sessions"

	"github.com/TravisS25/httputil/apiutil/apitest"
	"github.com/TravisS25/httputil/cacheutil/cachetest"

	"github.com/TravisS25/httputil/cacheutil"
	"github.com/pkg/errors"

	"github.com/TravisS25/httputil"
	"github.com/TravisS25/httputil/dbutil/dbtest"
)

const (
	decodeErr   = "decodeErr"
	internalErr = "internalErr"
	noRowsErr   = "noRowsErr"
	generalErr  = "generalErr"
	invalidJSON = "invalidJSON"

	cookieName = "user"
)

var (
	// This should be used for read only
	mUser = middlewareUser{
		ID:    "1",
		Email: "someemail@email.com",
	}

	// This should be used for read only
	mockHandler = &apitest.MockHandler{
		ServeHTTPFunc: func(w http.ResponseWriter, r *http.Request) {},
	}

	// This should be used for read only
	groupMap = map[string]bool{
		"Admin":   true,
		"Manager": false,
	}

	// This should be used for read only
	routingMap = map[string]bool{
		"/url1": true,
		"/url2": true,
	}
)

func getCacheFunc(key string) ([]byte, error) {
	if strings.Contains(key, "groups") {
		return json.Marshal(groupMap)
	}

	return json.Marshal(routingMap)
}
func getCacheFuncInvalidJSON(key string) ([]byte, error) {
	return json.Marshal([]string{"foo"})
}
func getCacheFuncErr(key string) ([]byte, error) {
	return nil, errors.New(generalErr)
}
func getCacheFuncNilErr(key string) ([]byte, error) {
	return nil, cacheutil.ErrCacheNil
}
func hasKeyCacheFunc(key string) (bool, error) {
	return false, nil
}

func getSessionFuncErr(r *http.Request, name string) (*sessions.Session, error) {
	return nil, errors.New("error")
}
func pingSessionFunc() (bool, error) {
	return true, nil
}
func pingSessionFuncErr() (bool, error) {
	return false, errors.New("error")
}
func saveSessionFunc(r *http.Request, w http.ResponseWriter, s *sessions.Session) error {
	return nil
}
func getFuncNewSession(r *http.Request, name string) (*sessions.Session, error) {
	ms := newDefaultMockSessionStore()
	s := sessions.NewSession(ms, cookieName)
	s.IsNew = true
	return s, nil
}
func getFuncSession(r *http.Request, name string) (*sessions.Session, error) {
	ms := newDefaultMockSessionStore()
	s := sessions.NewSession(ms, cookieName)
	s.IsNew = false
	return s, nil
}
func getFuncSessionWithValues(r *http.Request, name string) (*sessions.Session, error) {
	ms := newDefaultMockSessionStore()
	s := sessions.NewSession(ms, cookieName)
	u := mUser
	bUser, err := json.Marshal(&u)

	if err != nil {
		return s, err
	}

	s.Values[cookieName] = bUser
	return s, nil
}
func getFuncSessionWithInvalidValues(r *http.Request, name string) (*sessions.Session, error) {
	ms := newDefaultMockSessionStore()
	s := sessions.NewSession(ms, cookieName)
	foo := []string{"foo"}
	bUser, err := json.Marshal(&foo)

	if err != nil {
		return s, err
	}

	s.Values[cookieName] = bUser
	return s, nil
}

func newDefaultMockSessionStore() *cachetest.MockSessionStore {
	return &cachetest.MockSessionStore{
		GetFunc:  getFuncSession,
		NewFunc:  getFuncNewSession,
		PingFunc: pingSessionFunc,
		SaveFunc: saveSessionFunc,
	}
}

func TestAuthMiddleware(t *testing.T) {
	queryUser := "queryUser"
	querySession := "querySession"

	// getFuncErr := func(r *http.Request, name string) (*sessions.Session, error) {
	// 	return nil, errors.New("error")
	// }
	// pingFunc := func() (bool, error) {
	// 	return true, nil
	// }
	// pingFuncErr := func() (bool, error) {
	// 	return false, errors.New("error")
	// }
	// saveFunc := func(r *http.Request, w http.ResponseWriter, s *sessions.Session) error {
	// 	return nil
	// }
	mockSessionStore := &cachetest.MockSessionStore{
		GetFunc:  getSessionFuncErr,
		NewFunc:  getSessionFuncErr,
		PingFunc: pingSessionFuncErr,
		SaveFunc: saveSessionFunc,
	}
	// getFuncNewSession := func(r *http.Request, name string) (*sessions.Session, error) {
	// 	s := sessions.NewSession(mockSessionStore, cookieName)
	// 	s.IsNew = true
	// 	return s, nil
	// }
	// getFuncSession := func(r *http.Request, name string) (*sessions.Session, error) {
	// 	s := sessions.NewSession(mockSessionStore, cookieName)
	// 	s.IsNew = false
	// 	return s, nil
	// }
	// getFuncSessionWithValues := func(r *http.Request, name string) (*sessions.Session, error) {
	// 	s := sessions.NewSession(mockSessionStore, cookieName)
	// 	u := mUser
	// 	bUser, err := json.Marshal(&u)

	// 	if err != nil {
	// 		return s, err
	// 	}

	// 	s.Values[cookieName] = bUser
	// 	return s, nil
	// }
	// getFuncSessionWithInvalidValues := func(r *http.Request, name string) (*sessions.Session, error) {
	// 	s := sessions.NewSession(mockSessionStore, cookieName)
	// 	foo := []string{"foo"}
	// 	bUser, err := json.Marshal(&foo)

	// 	if err != nil {
	// 		return s, err
	// 	}

	// 	s.Values[cookieName] = bUser
	// 	return s, nil
	// }

	request := httptest.NewRequest(http.MethodGet, "/url", nil)
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
	queryForUser := func(w http.ResponseWriter, r *http.Request, db httputil.DBInterfaceV2) ([]byte, error) {
		if r.Header.Get(queryUser) == decodeErr {
			return nil, cachetest.NewMockSessionError(nil, "Decode cookie error", false, true, false)
		}

		if r.Header.Get(queryUser) == internalErr {
			fmt.Printf("made to internal error\n")
			return nil, cachetest.NewMockSessionError(nil, "Internal cookie error", false, false, true)
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

		u := mUser

		if r.Header.Get(querySession) == noRowsErr {
			u.ID = "0"
		}

		if r.Header.Get(querySession) == generalErr {
			u.ID = "-1"
		}

		return json.Marshal(&u)
	}

	authHandler := NewAuthHandler(mockDB, queryForUser, authConfig)

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
	mockSessionStore.PingFunc = pingSessionFunc
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

	// Setting up successfully getting session from
	// cache store that is not new but unable
	// to find user value within session
	mockSessionStore.GetFunc = getFuncSession
	authConfig.SessionStore = mockSessionStore
	authHandler.setConfig(authConfig)

	// Testing successful new session but without
	// finding user value in session
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Setting up for session to get value but have invalid json
	mockSessionStore.GetFunc = getFuncSessionWithInvalidValues
	authConfig.SessionStore = mockSessionStore

	// Testing json error occurs when trying to retrive user
	// info from session
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up for session to successfully get value and
	// have right user info
	mockSessionStore.GetFunc = getFuncSessionWithValues
	authConfig.SessionStore = mockSessionStore

	// Testing successful getting session from store
	// with right user info
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, request)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}
}

func TestGroupMiddleware(t *testing.T) {
	queryGroups := "queryGroups"

	req := httptest.NewRequest(http.MethodGet, "/url", nil)
	mockCache := &cachetest.MockCache{
		GetFunc:    getCacheFuncErr,
		HasKeyFunc: hasKeyCacheFunc,
	}
	mockDB := &dbtest.MockDB{
		RecoverErrorFunc: func(err error) bool {
			return true
		},
	}
	queryForGroups := func(w http.ResponseWriter, r *http.Request, db httputil.DBInterfaceV2) ([]byte, error) {
		if r.Header.Get(queryGroups) == noRowsErr {
			return nil, sql.ErrNoRows
		}

		if r.Header.Get(queryGroups) == generalErr {
			return nil, errors.New(generalErr)
		}

		if r.Header.Get(queryGroups) == invalidJSON {
			return json.Marshal([]string{"foo"})
		}

		// groupMap := map[string]bool{
		// 	"Admin":   true,
		// 	"Manager": false,
		// }
		groupBytes, err := json.Marshal(groupMap)

		if err != nil {
			return nil, err
		}

		return groupBytes, err
	}
	groupHandler := NewGroupHandler(
		mockDB,
		queryForGroups,
		GroupHandlerConfig{},
	)

	// Testing that if user is not logged in,
	// should continue to next handler
	rr := httptest.NewRecorder()
	h := groupHandler.MiddlewareFunc(mockHandler)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	u := mUser
	uBytes, err := json.Marshal(&u)

	if err != nil {
		t.Fatalf("%s\n", err.Error())
	}

	// Setting up and adding user and middleware user context
	// to request context for future requests so user will be
	// considered logged in
	ctx := req.Context()
	ctx = context.WithValue(ctx, UserCtxKey, uBytes)
	ctx = context.WithValue(ctx, MiddlewareUserCtxKey, u)
	req = req.WithContext(ctx)

	// Setting up where db returns sql.ErrNoRows
	req.Header.Set(queryGroups, noRowsErr)

	// Testing that user is logged in but db returns with
	// no rows which should just go to next handler
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Setting up where db returns error
	req.Header.Set(queryGroups, generalErr)

	// Testing that user is logged in but db returns
	// with an error
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up where db returns proper group map
	req.Header.Set(queryGroups, "")

	// Testing that user is logged in and
	// db returns proper map
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Setting up to use cache but fail and also
	// the db returns an error
	req.Header.Set(queryGroups, generalErr)
	groupHandler.config.CacheStore = mockCache

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up where we get cache nil err and
	// db also fails
	mockCache.GetFunc = getCacheFuncNilErr
	groupHandler.config.IgnoreCacheNil = true

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up where we don't ignore cache nil
	groupHandler.config.IgnoreCacheNil = false
	req.Header.Set(queryGroups, "")

	// Testing that our handler will skip db
	// and go to next handler
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
		t.Errorf("err: %s\n", rr.Body)
	}

	// Setting up for cache to return nil and then
	// for db to also fail
	req.Header.Set(queryGroups, generalErr)
	mockCache.GetFunc = getCacheFuncNilErr
	groupHandler.config.CacheStore = mockCache
	groupHandler.config.IgnoreCacheNil = true

	// Testing that cache returns nil and then db
	// also fails
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
		t.Errorf("err: %s\n", rr.Body)
	}

	// Setting up where we get cache err and
	// don't ignore nil so should continue to
	// next handler
	groupHandler.config.IgnoreCacheNil = false

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
		t.Errorf("err: %s\n", rr.Body)
	}

	// Setting up that cache returns successfully
	req.Header.Set(queryGroups, "")
	mockCache.GetFunc = getCacheFunc
	groupHandler.config.CacheStore = mockCache

	// Testing that cache returns successfully and returns
	// group map
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Setting up where db returns invalid json
	req.Header.Set(queryGroups, invalidJSON)
	groupHandler.config.CacheStore = nil

	// Testing that db returns invalid json resulting
	// in server error
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up where cache return invalid json
	// resulting in server error
	mockCache.GetFunc = getCacheFuncInvalidJSON
	groupHandler.config.CacheStore = mockCache

	// Testing cache returns invalid json
	// resulting in server error
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}
}

func TestRoutingMiddleware(t *testing.T) {
	queryRouting := "queryRouting"
	path := "path"
	invalidPath := "invalidPath"

	pathRegex := func(r *http.Request) (string, error) {
		if r.Header.Get(path) == invalidPath {
			return "invalidPath", nil
		}

		if r.Header.Get(path) == generalErr {
			return "", errors.New(generalErr)
		}

		return "/url1", nil
	}
	// pathRegexErr := func(r *http.Request) (string, error) {
	// 	return "", errors.New(generalErr)
	// }

	req := httptest.NewRequest(http.MethodOptions, "/url", nil)
	mockCache := &cachetest.MockCache{
		GetFunc:    getCacheFuncErr,
		HasKeyFunc: hasKeyCacheFunc,
	}
	mockDB := &dbtest.MockDB{
		RecoverErrorFunc: func(err error) bool {
			return true
		},
	}
	queryForRouting := func(w http.ResponseWriter, r *http.Request, db httputil.DBInterfaceV2) ([]byte, error) {
		if r.Header.Get(queryRouting) == noRowsErr {
			return nil, sql.ErrNoRows
		}

		if r.Header.Get(queryRouting) == generalErr {
			return nil, errors.New(generalErr)
		}

		if r.Header.Get(queryRouting) == invalidJSON {
			return json.Marshal([]string{"foo"})
		}

		routingBytes, err := json.Marshal(routingMap)

		if err != nil {
			return nil, err
		}

		return routingBytes, err
	}
	routingHandler := NewRoutingHandler(
		mockDB,
		queryForRouting,
		pathRegex,
		map[string]bool{
			"/url1": true,
		},
		RoutingHandlerConfig{},
	)

	// Testing that option request just simply
	// goes to the next handler
	rr := httptest.NewRecorder()
	h := routingHandler.MiddlewareFunc(mockHandler)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Testing that path regex returns err
	// (this should not really ever happen)
	req.Method = http.MethodGet
	req.Header.Set(path, generalErr)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up request where user is not logged in
	// so should use non user urls and be match
	req.Header.Set(path, "")
	routingHandler.pathRegex = pathRegex

	// Testing successful url find on non user
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Setting up invalid path return so user
	// should unauthorized error
	req.Header.Set(path, invalidPath)
	routingHandler.pathRegex = pathRegex

	// Testing user gets error trying to access url
	// not in non user map
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf(statusErrTxt, http.StatusForbidden, rr.Code)
	}

	// Adding middleware user to context of request
	// so it acts like user is logged in
	u := mUser
	ctx := req.Context()
	ctx = context.WithValue(ctx, MiddlewareUserCtxKey, u)
	req = req.WithContext(ctx)

	// Setting up where cache is not set
	// and db returns no rows error
	// which results in forbidden status
	req.Header.Set(queryRouting, noRowsErr)

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf(statusErrTxt, http.StatusForbidden, rr.Code)
	}

	// Setting up where cache is not and
	// db returns invalid json bytes resulting
	// in internal server err
	req.Header.Set(queryRouting, invalidJSON)

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up where cache is not set but
	// db returns results and path is allowed
	// which should result in ok status
	req.Header.Set(queryRouting, "")
	req.Header.Set(path, "")

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}

	// Setting up where cache is set but returns
	// error and db also returns error
	req.Header.Set(queryRouting, generalErr)
	routingHandler.config.CacheStore = mockCache

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up where cache returns nil error
	// and we ignore the nil error and query db
	// which also returns an error
	mockCache.GetFunc = getCacheFuncNilErr
	routingHandler.config.CacheStore = mockCache
	routingHandler.config.IgnoreCacheNil = true

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up here cache nil error is not ignored
	// so we simply give forbidden error
	routingHandler.config.IgnoreCacheNil = false

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf(statusErrTxt, http.StatusForbidden, rr.Code)
	}

	// Setting up where we get url bytes from cache
	// and but it's invalid json
	mockCache.GetFunc = getCacheFuncInvalidJSON

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf(statusErrTxt, http.StatusInternalServerError, rr.Code)
	}

	// Setting up where we get cache and is valid
	// to move on to the next handler
	mockCache.GetFunc = getCacheFunc

	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf(statusErrTxt, http.StatusOK, rr.Code)
	}
}
