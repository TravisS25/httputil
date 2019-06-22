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

	"github.com/go-redis/redis"
	"github.com/urfave/negroni"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"

	"github.com/TravisS25/httputil"

	"github.com/TravisS25/httputil/cacheutil"
)

// Query types to be used against the Middleware#QueryDB function
const (
	UserQuery = iota
	GroupQuery
	RoutingQuery
	SessionQuery
)

const (
	// GroupKey is used as a key when pulling a user's groups out from cache
	GroupKey = "%s-groups"

	// URLKey is used as a key when pulling a user's allowed urls from cache
	URLKey = "%s-urls"
)

var (
	UserCtxKey           = MiddlewareKey{KeyName: "user"}
	GroupCtxKey          = MiddlewareKey{KeyName: "groupName"}
	MiddlewareUserCtxKey = MiddlewareKey{KeyName: "middlewareUser"}
)

type MiddlewareKey struct {
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

type QueryDB func(res *http.Request, db httputil.DBInterface) ([]byte, error)

type Middleware struct {
	CacheStore   cacheutil.CacheStore
	SessionStore cacheutil.SessionStore
	DB           httputil.DBInterface
	LogInserter  func(res http.ResponseWriter, req *http.Request, payload []byte, db httputil.DBInterface) error
	QueryDB      func(res *http.Request, db httputil.DBInterface, queryType int) ([]byte, error)
	AnonRouting  []string

	SessionKeys *cacheutil.SessionConfig
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
	var session *sessions.Session
	var err error

	if m.SessionKeys == nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	session, err = m.SessionStore.Get(r, m.SessionKeys.SessionName)

	if err != nil {
		fmt.Printf("no session err: %s\n", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// If session is considered new, that means
	// either current user is truly not logged in or cache was/is down
	if session.IsNew {
		// fmt.Printf("new session\n")

		// First we determine if user is sending a cookie with our user cookie key
		// If they are, try retrieving from db if Middleware#QueryDB is set
		if _, err := r.Cookie(m.SessionKeys.SessionName); err == nil {
			fmt.Printf("has cookie but not found in store\n")
			if m.DB != nil && m.QueryDB != nil {
				fmt.Printf("auth middleware db\n")
				userBytes, err := m.QueryDB(r, m.DB, UserQuery)

				if err != nil {
					switch err.(type) {
					case securecookie.Error:
						w.WriteHeader(http.StatusBadRequest)
					default:
						if err == sql.ErrNoRows {
							w.WriteHeader(http.StatusBadRequest)
						} else {
							w.WriteHeader(http.StatusInternalServerError)
						}
					}
					return
				}

				err = json.Unmarshal(userBytes, &middlewareUser)

				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}

				// Here we test to see if our session backend is responsive
				// If it is, that means current user logged in while cache was down
				// and was using the database to grab their sessions but since session
				// backend is back up, we can grab current user's session from
				// database and set it to session backend and use that instead of database
				// for future requests
				if _, err = m.SessionStore.Ping(); err == nil {
					fmt.Printf("ping successful\n")
					sessionIDBytes, err := m.QueryDB(r, m.DB, SessionQuery)

					if err != nil {
						if err == sql.ErrNoRows {
							fmt.Printf("auth middleware db no row found\n")
							next(w, r)
							return
						}

						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					fmt.Printf("session bytes: %s\n", sessionIDBytes)

					session, _ = m.SessionStore.New(r, m.SessionKeys.SessionName)
					session.ID = string(sessionIDBytes)
					fmt.Printf("session id: %s\n", session.ID)
					session.Values[m.SessionKeys.UserKey] = userBytes
					session.Save(r, w)
					fmt.Printf("set session into store \n")
				}

				ctx := context.WithValue(r.Context(), UserCtxKey, userBytes)
				ctxWithEmail := context.WithValue(ctx, MiddlewareUserCtxKey, middlewareUser)
				next(w, r.WithContext(ctxWithEmail))
			} else {
				next(w, r)
			}
		} else {
			// fmt.Printf("new session, no cookie\n")
			next(w, r)
		}
	} else {
		if val, ok := session.Values[m.SessionKeys.UserKey]; ok {
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
			if err != redis.Nil {
				if m.DB != nil && m.QueryDB != nil {
					fmt.Printf("group middleware db\n")
					groupBytes, err = m.QueryDB(r, m.DB, GroupQuery)

					if err != nil {
						if err == sql.ErrNoRows {
							fmt.Printf("group middleware db no row found\n")
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

// RoutingMiddleware is middleware used to indicate whether an incoming request is
// authorized to go to certain urls based on authentication of a user's groups
// The groups should come from cache and be deserialzed into an array of strings
// and if current requested url matches any of the urls, they are allowed forward,
// if not, we 404
//
// Middleware#CacheStore and Middleware#AnonRouting must be set to use
// Middleware#AuthMiddleware and Middleware#GroupMiddleware must be before this middleware
func (m *Middleware) RoutingMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	rootPath := "/"
	path := r.URL.Path
	allowedPath := false

	if r.Method != http.MethodOptions {
		if r.Context().Value(MiddlewareUserCtxKey) != nil {
			user := r.Context().Value(MiddlewareUserCtxKey).(middlewareUser)
			key := fmt.Sprintf(URLKey, user.Email)
			urlBytes, err := m.CacheStore.Get(key)

			if err != nil {
				if err != redis.Nil {
					if m.DB != nil && m.QueryDB != nil {
						fmt.Printf("routing middleware db\n")
						urlBytes, err = m.QueryDB(r, m.DB, RoutingQuery)

						if err != nil {
							if err == sql.ErrNoRows {
								fmt.Printf("routing middleware db no row found\n")
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

// -----------------------------------------------------------

type AuthHandler struct {
	handler      http.Handler
	sessionStore cacheutil.SessionStore
	db           httputil.DBInterface
	queryDB      QueryDB
	sessionKeys  *cacheutil.SessionConfig
}

func NewAuthHandler(
	//handler http.Handler,
	db httputil.DBInterface,
	queryDB QueryDB,
	sessionStore cacheutil.SessionStore,
	sessionKeys *cacheutil.SessionConfig,
) *AuthHandler {
	return &AuthHandler{
		//handler:      handler,
		sessionStore: sessionStore,
		sessionKeys:  sessionKeys,
		db:           db,
		queryDB:      queryDB,
	}
}

func (a *AuthHandler) MiddlewareFunc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var middlewareUser middlewareUser
		var session *sessions.Session
		var err error

		if a.sessionKeys == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		session, err = a.sessionStore.Get(r, a.sessionKeys.SessionName)

		if err != nil {
			fmt.Printf("no session err: %s\n", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// If session is considered new, that means
		// either current user is truly not logged in or cache was/is down
		if session.IsNew {
			// fmt.Printf("new session\n")

			// First we determine if user is sending a cookie with our user cookie key
			// If they are, try retrieving from db if Middleware#QueryDB is set
			if _, err := r.Cookie(a.sessionKeys.SessionName); err == nil {
				fmt.Printf("has cookie but not found in store\n")
				if a.db != nil && a.queryDB != nil {
					fmt.Printf("auth middleware db\n")
					userBytes, err := a.queryDB(r, a.db)

					if err != nil {
						switch err.(type) {
						case securecookie.Error:
							w.WriteHeader(http.StatusBadRequest)
						default:
							if err == sql.ErrNoRows {
								w.WriteHeader(http.StatusBadRequest)
							} else {
								w.WriteHeader(http.StatusInternalServerError)
							}
						}
						return
					}

					err = json.Unmarshal(userBytes, &middlewareUser)

					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte(err.Error()))
						return
					}

					// Here we test to see if our session backend is responsive
					// If it is, that means current user logged in while cache was down
					// and was using the database to grab their sessions but since session
					// backend is back up, we can grab current user's session from
					// database and set it to session backend and use that instead of database
					// for future requests
					if _, err = a.sessionStore.Ping(); err == nil {
						fmt.Printf("ping successful\n")
						sessionIDBytes, err := a.queryDB(r, a.db)

						if err != nil {
							if err == sql.ErrNoRows {
								fmt.Printf("auth middleware db no row found\n")
								next.ServeHTTP(w, r)
								return
							}

							w.WriteHeader(http.StatusInternalServerError)
							return
						}

						fmt.Printf("session bytes: %s\n", sessionIDBytes)

						session, _ = a.sessionStore.New(r, a.sessionKeys.SessionName)
						session.ID = string(sessionIDBytes)
						fmt.Printf("session id: %s\n", session.ID)
						session.Values[a.sessionKeys.UserKey] = userBytes
						session.Save(r, w)
						fmt.Printf("set session into store \n")
					}

					ctx := context.WithValue(r.Context(), UserCtxKey, userBytes)
					ctxWithEmail := context.WithValue(ctx, MiddlewareUserCtxKey, middlewareUser)
					next.ServeHTTP(w, r.WithContext(ctxWithEmail))
				} else {
					next.ServeHTTP(w, r)
				}
			} else {
				// fmt.Printf("new session, no cookie\n")
				next.ServeHTTP(w, r)
			}
		} else {
			if val, ok := session.Values[a.sessionKeys.UserKey]; ok {
				userBytes := val.([]byte)

				err := json.Unmarshal(val.([]byte), &middlewareUser)

				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}

				ctx := context.WithValue(r.Context(), UserCtxKey, userBytes)
				ctxWithEmail := context.WithValue(ctx, MiddlewareUserCtxKey, middlewareUser)
				next.ServeHTTP(w, r.WithContext(ctxWithEmail))
			} else {
				next.ServeHTTP(w, r)
			}
		}
	})
}

type GroupHandler struct {
	//handler    http.Handler
	cacheStore cacheutil.CacheStore
	db         httputil.DBInterface
	queryDB    QueryDB
}

func NewGroupHandler(
	//handler http.Handler,
	db httputil.DBInterface,
	queryDB QueryDB,
	cacheStore cacheutil.CacheStore,
) *GroupHandler {
	return &GroupHandler{
		//handler:    handler,
		cacheStore: cacheStore,
		db:         db,
		queryDB:    queryDB,
	}
}

func (g *GroupHandler) MiddlewareFunc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//fmt.Printf("group middleware\n")
		if r.Context().Value(MiddlewareUserCtxKey) != nil {
			var groupMap map[string]bool

			user := r.Context().Value(MiddlewareUserCtxKey).(middlewareUser)
			groups := fmt.Sprintf(GroupKey, user.Email)
			groupBytes, err := g.cacheStore.Get(groups)

			if err != nil {
				if err != redis.Nil {
					if g.db != nil && g.queryDB != nil {
						fmt.Printf("group middleware db\n")
						groupBytes, err = g.queryDB(r, g.db)

						if err != nil {
							if err == sql.ErrNoRows {
								fmt.Printf("group middleware db no row found\n")
								next.ServeHTTP(w, r)
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
					next.ServeHTTP(w, r)
					return
				}
			}

			json.Unmarshal(groupBytes, &groupMap)
			ctx := context.WithValue(r.Context(), GroupCtxKey, groupMap)

			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			//fmt.Printf("no user group middleware\n")
			next.ServeHTTP(w, r)
		}
	})
}

type RoutingHandler struct {
	//handler     http.Handler
	cacheStore  cacheutil.CacheStore
	db          httputil.DBInterface
	queryDB     QueryDB
	pathRegex   httputil.PathRegex
	nonUserURLs map[string]bool
}

func NewRoutingHandler(
	//handler http.Handler,
	db httputil.DBInterface,
	queryDB QueryDB,
	cacheStore cacheutil.CacheStore,
	pathRegex httputil.PathRegex,
	nonUserURLs map[string]bool,
) *RoutingHandler {
	return &RoutingHandler{
		//handler:     handler,
		cacheStore:  cacheStore,
		db:          db,
		queryDB:     queryDB,
		pathRegex:   pathRegex,
		nonUserURLs: nonUserURLs,
	}
}

func (routing *RoutingHandler) MiddlewareFunc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /fmt.Printf("routing middleware\n")
		if r.Method != http.MethodOptions {
			var urlBytes []byte
			var err, cacheErr error

			user := r.Context().Value(UserCtxKey)
			pathExp, _ := routing.pathRegex(r)
			allowedPath := false

			if user != nil {
				user := r.Context().Value(MiddlewareUserCtxKey).(middlewareUser)
				key := fmt.Sprintf(URLKey, user.Email)
				urlBytes, cacheErr = routing.cacheStore.Get(key)

				if cacheErr != nil {
					if err != redis.Nil {
						if routing.db != nil && routing.queryDB != nil {
							fmt.Printf("routing middleware db\n")
							urlBytes, err = routing.queryDB(r, routing.db)

							if err != nil {
								if err == sql.ErrNoRows {
									fmt.Printf("routing middleware db no row found\n")
									next.ServeHTTP(w, r)
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
						next.ServeHTTP(w, r)
						return
					}
				}

				var urls map[string]bool
				err = json.Unmarshal(urlBytes, &urls)

				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(err.Error()))
					return
				}

				if _, ok := urls[pathExp]; ok {
					allowedPath = true
				}
			} else {
				if _, ok := routing.nonUserURLs[pathExp]; ok {
					allowedPath = true
				}
			}

			if !allowedPath {
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte("Not authorized to access url"))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
