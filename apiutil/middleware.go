package apiutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/urfave/negroni"

	"github.com/gorilla/sessions"

	"github.com/TravisS25/httputil"

	"github.com/TravisS25/httputil/cacheutil"
)

const (
	// GroupKey is used as a key when pulling a user's groups out from cache
	GroupKey = "%s-groups"

	// URLKey is used as a key when pulling a user's allowed urls from cache
	URLKey = "%s-urls"
)

var (
	userCtxKey  = key{KeyName: "user"}
	groupCtxKey = key{KeyName: "groupName"}
	emailCtxKey = key{KeyName: "email"}
)

type key struct {
	KeyName string
}

// IUser is the interface that must be implemented by your user session struct
// to use GroupMiddleware and RoutingMiddleware
type IUser interface {
	GetID() string
	GetEmail() string
}

type email struct {
	Email string `json:"email"`
}

// InsertLogger is interface that allows to log user's actions of
// post, put, or delete request
// This is used in conjunction with Middleware#LogEntryMiddleware
type InsertLogger interface {
	InsertLog(r *http.Request, payload string, db httputil.DBInterface) error
}

type Middleware struct {
	CacheStore      cacheutil.CacheStore
	SessionStore    sessions.Store
	DB              httputil.DBInterface
	Inserter        InsertLogger
	UserSessionName string
	AnonRouting     []string
}

// LogEntryMiddleware is used for logging a user modifying actions such as put, post, and delete
func (m *Middleware) LogEntryMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	var payload interface{}
	var err error
	rw := negroni.NewResponseWriter(w)

	if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
		if r.Body != nil {
			body, _ := ioutil.ReadAll(r.Body)
			r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
			dec := json.NewDecoder(bytes.NewBuffer(body))
			err = dec.Decode(&payload)
		}
	}

	next(rw, r)

	if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
		if (rw.Status() == 0 || rw.Status() == 200) && err == nil {
			jsonBytes, err := json.Marshal(payload)

			if err != nil {
				panic("error inserting log into db")
			}

			m.Inserter.InsertLog(r, string(jsonBytes), m.DB)
		} else if rw.Status() == 0 || rw.Status() == 200 {
			m.Inserter.InsertLog(r, "", m.DB)
		}
	}
}

// AuthMiddleware is middleware used to check for authenication of incoming requests
// If there is a session for a user for current request, we add this to the context of the request
// If you plan on using other middleware of this middleware class, your unmarshaled user
// must implement the IUser interface
//
// Middleware#SessionStore must be set in order to use
func (m *Middleware) AuthMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if m.UserSessionName == "" {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("User session not set"))
		return
	}
	session, _ := m.SessionStore.Get(r, m.UserSessionName)

	if val, ok := session.Values[m.UserSessionName]; ok {
		var user interface{}
		var email email
		err := json.Unmarshal(val.([]byte), &user)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = json.Unmarshal(val.([]byte), &email)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), userCtxKey, &user)
		ctxWithEmail := context.WithValue(ctx, emailCtxKey, email.Email)
		next(w, r.WithContext(ctxWithEmail))
	} else {
		next(w, r)
	}
}

// GroupMiddleware is middleware used to add current user's groups (if logged in) to the context
// of the request
// User session struct must implement IUser interface to use GroupMiddleware
//
// Middleware#CacheStore must be set in order to use
// Middleware#AuthMiddleware must come before this middleware
func (m *Middleware) GroupMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if r.Context().Value(emailCtxKey) != nil {
		var groupArray []string
		email := r.Context().Value(emailCtxKey).(string)

		groups := fmt.Sprintf(GroupKey, email)
		groupBytes, _ := m.CacheStore.Get(groups)
		json.Unmarshal(groupBytes, &groupArray)
		ctx := context.WithValue(r.Context(), groupCtxKey, groupArray)
		next(w, r.WithContext(ctx))
	} else {
		next(w, r)
	}
}

// RoutingMiddleware is middleware used to indicate whether an incoming request is
// authorized to go to certain urls based on authentication of a user's groups
// The groups should come from cache and be deserialzed into an array of strings
// and if current requested url matches any of the urls, they are allowed forward,
// in not, we 404
//
// Middleware#CacheStore and Middleware#AnonRouting must be set to use
// Middleware#AuthMiddleware and Middleware#GroupMiddleware must be before this middleware
func (m *Middleware) RoutingMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	rootPath := "/"
	path := r.URL.Path
	allowedPath := false

	if r.Method != "OPTIONS" {
		if r.Context().Value(emailCtxKey) != nil {
			email := r.Context().Value(emailCtxKey).(string)
			key := fmt.Sprintf(URLKey, email)
			urlBytes, err := m.CacheStore.Get(key)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			var urls []string
			err = json.Unmarshal(urlBytes, &urls)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if path == rootPath {
				allowedPath = true
			}

			for _, url := range urls {
				if strings.Contains(path, url) && url != rootPath {
					allowedPath = true
					break
				}
			}

		} else {
			if path == rootPath {
				allowedPath = true
			} else {
				for _, url := range m.AnonRouting {
					if strings.Contains(path, url) && url != rootPath {
						allowedPath = true
						break
					}
				}
			}
		}

		if !allowedPath {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Not authorized to access url"))
			return
		}

	}

	next(w, r)
}

// ----------------------------------------------------

type middleware struct {
	cacheStore      cacheutil.CacheStore
	sessionStore    sessions.Store
	db              httputil.DBInterface
	inserter        InsertLogger
	userSessionName string
	anonRouting     []string
}

// NewMiddleware is init function for middleware
func NewMiddleware(sessionStore sessions.Store, cacheStore cacheutil.CacheStore, db httputil.DBInterface, anonRouting []string) *middleware {
	return &middleware{
		cacheStore:      cacheStore,
		sessionStore:    sessionStore,
		anonRouting:     anonRouting,
		db:              db,
		userSessionName: "user",
	}
}

func (m *middleware) SetUserSessionName(name string) {
	m.userSessionName = name
}

// func (m *middleware) LogEntryMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
// 	var payload interface{}
// 	var err error
// 	rw := negroni.NewResponseWriter(w)

// 	if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
// 		if r.Body != nil {
// 			body, _ := ioutil.ReadAll(r.Body)
// 			r.Body = ioutil.NopCloser(bytes.NewBuffer(body))
// 			dec := json.NewDecoder(bytes.NewBuffer(body))
// 			err = dec.Decode(&payload)
// 		}
// 	}

// 	next(rw, r)

// 	if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
// 		if (rw.Status() == 0 || rw.Status() == 200) && err == nil {
// 			m.inserter.InsertLog(r, payload, m.db)
// 		} else {
// 			m.inserter.InsertLog(r, nil, m.db)
// 		}
// 	}
// }

// Authmiddleware is middleware used to check for authenication of incoming requests
// If there is a session for a user for current request, we add this to the context of the request
// If you plan on using other middleware of this middleware class, your unmarshaled user
// must implement the IUser interface
func (m *middleware) AuthMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	session, _ := m.sessionStore.Get(r, m.userSessionName)

	if val, ok := session.Values[m.userSessionName]; ok {
		var user interface{}
		err := json.Unmarshal(val.([]byte), &user)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), userCtxKey, &user)
		next(w, r.WithContext(ctx))
	} else {
		next(w, r)
	}
}

// GroupMiddleware is middleware used to add current user's groups (if logged in) to the context
// of the request.  User session struct must implement IUser interface
func (m *middleware) GroupMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	user := GetUser(r)

	if user != nil {
		var groupArray []string
		currentUser := user.(IUser)
		groups := fmt.Sprintf(GroupKey, currentUser.GetEmail())
		groupBytes, _ := m.cacheStore.Get(groups)
		json.Unmarshal(groupBytes, &groupArray)
		ctx := context.WithValue(r.Context(), groupCtxKey, groupArray)
		next(w, r.WithContext(ctx))
	} else {
		next(w, r)
	}
}

// RoutingMiddleware is middleware used to indicate whether an incoming request is
// authorized to go to certain urls based on authentication of a user's groups
// The groups should come from cache and be deserialzed into an array of strings
// and if current requested url matches any of the urls, they are allowed forward,
// in not, we 404
func (m *middleware) RoutingMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	rootPath := "/"
	path := r.URL.Path
	allowedPath := false
	user := GetUser(r)

	if r.Method != "OPTIONS" {
		if user != nil {
			currentUser := user.(IUser)
			key := fmt.Sprintf(URLKey, currentUser.GetEmail())
			urlBytes, err := m.cacheStore.Get(key)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			var urls []string
			err = json.Unmarshal(urlBytes, &urls)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if path == rootPath {
				allowedPath = true
			}

			for _, url := range urls {
				if strings.Contains(path, url) && url != rootPath {
					allowedPath = true
					break
				}
			}

		} else {
			if path == rootPath {
				allowedPath = true
			} else {
				for _, url := range m.anonRouting {
					if strings.Contains(path, url) && url != rootPath {
						allowedPath = true
						break
					}
				}
			}
		}

		if !allowedPath {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Not authorized to access url"))
			return
		}

		next(w, r)
	} else {
		next(w, r)
	}
}
