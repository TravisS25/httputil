package apiutil

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"

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
)

type key struct {
	KeyName string
}

// IUser is the interface that must be implemented by your user session struct
// to use GroupMiddleware and RoutingMiddleware
type IUser interface {
	Email() string
}

type middleware struct {
	cacheStore   cacheutil.CacheStore
	sessionStore sessions.Store
	//user            IUser
	userSessionName string
	anonRouting     []string
}

// NewMiddleware is init function for middleware
func NewMiddleware(sessionStore sessions.Store, cacheStore cacheutil.CacheStore, anonRouting []string) *middleware {
	return &middleware{
		cacheStore:      cacheStore,
		sessionStore:    sessionStore,
		anonRouting:     anonRouting,
		userSessionName: "user",
	}
}

func (m *middleware) SetUserSessionName(name string) {
	m.userSessionName = name
}

func (m *middleware) CSRFTokenmiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	w.Header().Set("X-CSRF-TOKEN", csrf.Token(r))
	next(w, r)
}

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
	user := GetUser(r).(IUser)

	if user != nil {
		var groupArray []string
		groups := fmt.Sprintf(GroupKey, user.Email())
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
// The groups shoud come from cache and be deserialzed into an array of strings
// and if current requested url matches any of the urls, they are allowed forward,
// in not, we 404
func (m *middleware) RoutingMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	rootPath := "/"
	path := r.URL.Path
	allowedPath := false
	user := GetUser(r).(IUser)

	if r.Method != "OPTIONS" {
		if user != nil {
			key := fmt.Sprintf(URLKey, user.Email())
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
			w.WriteHeader(http.StatusNotFound)
			return
		}

		next(w, r)
	} else {
		next(w, r)
	}
}
