package apiutil

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/garyburd/redigo/redis"

	"github.com/urfave/negroni"

	"github.com/gorilla/sessions"

	"github.com/TravisS25/httputil"

	"github.com/TravisS25/httputil/cacheutil"
)

// Query types to be used against the Middleware#QueryDB function
const (
	AuthMiddleware = iota
	GroupMiddleware
	RoutingMiddleware
)

const (
	// GroupKey is used as a key when pulling a user's groups out from cache
	GroupKey = "%s-groups"

	//GroupIDKey = "%s-groupIDs"

	// URLKey is used as a key when pulling a user's allowed urls from cache
	URLKey = "%s-urls"
)

var (
	UserCtxKey  = key{KeyName: "user"}
	GroupCtxKey = key{KeyName: "groupName"}
	// GroupIDCtxKey        = key{KeyName: "groupID"}
	MiddlewareUserCtxKey = key{KeyName: "middlewareUser"}
)

type key struct {
	KeyName string
}

type middlewareUser struct {
	ID    string `json:"id"`
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
	LogInserter     func(res http.ResponseWriter, req *http.Request, payload []byte, db httputil.DBInterface) error
	UserSessionName string
	QueryDB         func(res *http.Request, db httputil.DBInterface, queryType int) ([]byte, error)
	AnonRouting     []string
}

// LogEntryMiddleware is used for logging a user modifying actions such as put, post, and delete
func (m *Middleware) LogEntryMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	var payload []byte
	var err error
	rw := negroni.NewResponseWriter(w)

	if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
		if r.Body != nil {
			payload, _ = ioutil.ReadAll(r.Body)
			r.Body = ioutil.NopCloser(bytes.NewBuffer(payload))
		}
	}

	next(rw, r)

	if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
		if rw.Status() == 0 || rw.Status() == 200 {
			err = m.LogInserter(w, r, payload, m.DB)

			if HasServerError(w, err, "") {
				return
			}
		}
	}
}

// AuthMiddleware is middleware used to check for authenication of incoming requests
// If there is a session for a user for current request, we add this to the context of the request
// If you plan on using other middleware of this middleware class, your unmarshaled user
// must have the same fields as middlewareUser struct
//
// Middleware#SessionStore and Middleware#UserSessionName must be set in order to use
// Optionally if Middleware#DB and Middleware#UserSessionFunc is also set, it will resort
// to a database backend if cache fails if you are storing session related things in a database
// Middleware#UserSessionFunc should return json format of user in bytes
func (m *Middleware) AuthMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	var middlewareUser middlewareUser

	if m.UserSessionName == "" {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("User session not set"))
		return
	}
	session, err := m.SessionStore.Get(r, m.UserSessionName)

	fmt.Printf("session val: %v\n", session)

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err != nil {
		fmt.Printf("auth middleware err: %s\n", err.Error())
		if m.DB != nil && m.QueryDB != nil {
			fmt.Printf("auth middleware db")
			userBytes, err := m.QueryDB(r, m.DB, AuthMiddleware)

			if err != nil {
				if err == sql.ErrNoRows {
					next(w, r)
					return
				}

				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			err = json.Unmarshal(userBytes, &middlewareUser)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

			ctx := context.WithValue(r.Context(), UserCtxKey, userBytes)
			ctxWithEmail := context.WithValue(ctx, MiddlewareUserCtxKey, middlewareUser)
			next(w, r.WithContext(ctxWithEmail))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		if val, ok := session.Values[m.UserSessionName]; ok {
			userBytes := val.([]byte)

			err := json.Unmarshal(val.([]byte), &middlewareUser)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

			ctx := context.WithValue(r.Context(), UserCtxKey, userBytes)
			ctxWithEmail := context.WithValue(ctx, MiddlewareUserCtxKey, middlewareUser)
			next(w, r.WithContext(ctxWithEmail))
		} else {
			next(w, r)
		}
	}
}

// GroupMiddleware is middleware used to add current user's groups (if logged in) to the context
// of the request
// User session struct must have same fields as middlewarewUser
//
// Middleware#CacheStore must be set in order to use
// Middleware#AuthMiddleware must come before this middleware
func (m *Middleware) GroupMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if r.Context().Value(MiddlewareUserCtxKey) != nil {
		var groupArray []string

		user := r.Context().Value(MiddlewareUserCtxKey).(middlewareUser)
		groups := fmt.Sprintf(GroupKey, user.Email)
		groupBytes, err := m.CacheStore.Get(groups)

		if err != nil {
			if err != redis.ErrNil {
				if m.DB != nil && m.QueryDB != nil {
					fmt.Printf("group middleware db")
					groupBytes, err = m.QueryDB(r, m.DB, GroupMiddleware)

					if err != nil {
						if err == sql.ErrNoRows {
							next(w, r)
							return
						}

						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			} else {
				next(w, r)
				return
			}
		}

		json.Unmarshal(groupBytes, &groupArray)
		ctx := context.WithValue(r.Context(), GroupCtxKey, groupArray)

		next(w, r.WithContext(ctx))
	} else {
		next(w, r)
	}
}

// func (m *Middleware) GroupIDMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
// 	if r.Context().Value(MiddlewareUserCtxKey) != nil {
// 		var groupIDArray []int

// 		user := r.Context().Value(MiddlewareUserCtxKey).(middlewareUser)
// 		groupIDs := fmt.Sprintf(GroupIDKey, user.Email)
// 		groupIDBytes, _ := m.CacheStore.Get(groupIDs)
// 		json.Unmarshal(groupIDBytes, &groupIDArray)
// 		ctx := context.WithValue(r.Context(), GroupIDCtxKey, groupIDArray)
// 		next(w, r.WithContext(ctx))
// 	} else {
// 		next(w, r)
// 	}
// }

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
		if r.Context().Value(MiddlewareUserCtxKey) != nil {
			user := r.Context().Value(MiddlewareUserCtxKey).(middlewareUser)
			key := fmt.Sprintf(URLKey, user.Email)
			urlBytes, err := m.CacheStore.Get(key)

			if err != nil {
				if err != redis.ErrNil {
					if m.DB != nil && m.QueryDB != nil {
						fmt.Printf("routing middleware db")
						urlBytes, err = m.QueryDB(r, m.DB, RoutingMiddleware)

						if err != nil {
							if err == sql.ErrNoRows {
								next(w, r)
								return
							}

							w.WriteHeader(http.StatusInternalServerError)
							return
						}
					} else {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}
				} else {
					next(w, r)
					return
				}
			}

			var urls []string
			err = json.Unmarshal(urlBytes, &urls)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
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

		ctx := context.WithValue(r.Context(), UserCtxKey, &user)
		next(w, r.WithContext(ctx))
	} else {
		next(w, r)
	}
}

// GroupMiddleware is middleware used to add current user's groups (if logged in) to the context
// of the request.  User session struct must implement IUser interface
// func (m *middleware) GroupMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
// 	user := GetUser(r)

// 	if user != nil {
// 		var groupArray []string
// 		currentUser := user.(IUser)
// 		groups := fmt.Sprintf(GroupKey, currentUser.GetEmail())
// 		groupBytes, _ := m.cacheStore.Get(groups)
// 		json.Unmarshal(groupBytes, &groupArray)
// 		ctx := context.WithValue(r.Context(), GroupCtxKey, groupArray)
// 		next(w, r.WithContext(ctx))
// 	} else {
// 		next(w, r)
// 	}
// }

// RoutingMiddleware is middleware used to indicate whether an incoming request is
// authorized to go to certain urls based on authentication of a user's groups
// The groups should come from cache and be deserialzed into an array of strings
// and if current requested url matches any of the urls, they are allowed forward,
// in not, we 404
// func (m *middleware) RoutingMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
// 	rootPath := "/"
// 	path := r.URL.Path
// 	allowedPath := false
// 	user := GetUser(r)

// 	if r.Method != "OPTIONS" {
// 		if user != nil {
// 			currentUser := user.(IUser)
// 			key := fmt.Sprintf(URLKey, currentUser.GetEmail())
// 			urlBytes, err := m.cacheStore.Get(key)

// 			if err != nil {
// 				w.WriteHeader(http.StatusInternalServerError)
// 				return
// 			}

// 			var urls []string
// 			err = json.Unmarshal(urlBytes, &urls)

// 			if err != nil {
// 				w.WriteHeader(http.StatusInternalServerError)
// 				return
// 			}

// 			if path == rootPath {
// 				allowedPath = true
// 			}

// 			for _, url := range urls {
// 				if strings.Contains(path, url) && url != rootPath {
// 					allowedPath = true
// 					break
// 				}
// 			}

// 		} else {
// 			if path == rootPath {
// 				allowedPath = true
// 			} else {
// 				for _, url := range m.anonRouting {
// 					if strings.Contains(path, url) && url != rootPath {
// 						allowedPath = true
// 						break
// 					}
// 				}
// 			}
// 		}

// 		if !allowedPath {
// 			w.WriteHeader(http.StatusForbidden)
// 			w.Write([]byte("Not authorized to access url"))
// 			return
// 		}

// 		next(w, r)
// 	} else {
// 		next(w, r)
// 	}
// }
