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

const (
	serverErrTxt       = "Server error"
	unauthorizedURLTxt = "Not authorized to access url"
	invalidCookieTxt   = "Invalid cookie"
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

// HTTPResponseConfig is used to give default header and response
// values of an http request
// This will mainly be used for middleware config structs
// to allow user of middleware more control on what gets
// send back to the user
type HTTPResponseConfig struct {
	HTTPStatus   *int
	HTTPResponse []byte
}

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

type QueryDB func(w http.ResponseWriter, res *http.Request, db httputil.DBInterfaceV2) ([]byte, error)

func setHTTPResponseDefaults(config *HTTPResponseConfig, defaultStatus int, defaultResponse []byte) {
	if config.HTTPStatus == nil {
		config.HTTPStatus = &defaultStatus
	}
	if config.HTTPResponse == nil {
		config.HTTPResponse = defaultResponse
	}
}

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
					session.Values[m.SessionKeys.Keys.UserKey] = userBytes
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
		if val, ok := session.Values[m.SessionKeys.Keys.UserKey]; ok {
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

// AuthHandlerConfig is used as config struct for AuthHandler
// These settings are not required but if user wants to use things
// like a different session store besides a database, these should
// be set
type AuthHandlerConfig struct {
	// SessionStore is used to implement a backend to store sessions
	// besides a database like file system or in-memory database
	// i.e. Redis
	SessionStore cacheutil.SessionStore

	// SessionKeys is just an arbitrary set of common key names to store
	// in a session values
	SessionConfig cacheutil.SessionConfig

	// QueryForSession is used for inserting a session value from a database
	// to the entity that implements SessionStore
	// This is used in the case where a person logs in while the entity that
	// implements SessionStore is down and must query session from database
	//
	// If this is set, the implementing function should return the session id
	// from a database which will then be set to SessionStore if/when it comes back up
	//
	// This is bascially a recovery method if implementing SessionStore ever
	// goes down or some how gets its values flushed
	QueryForSession func(w http.ResponseWriter, db httputil.DBInterfaceV2, userID string) (sessionID string, err error)

	// DecodeCookieErrResponse is config used to respond to user if decoding
	// a cookie is invalid
	// This usually happens when a user sends an invalid cookie on request
	//
	// Default status value is http.StatusBadRequest
	// Default response value is []byte("Invalid cookie")
	DecodeCookieErrResponse HTTPResponseConfig

	// ServerErrResponse is config used to respond to user if some type
	// of server error occurs
	//
	// Default status value is http.StatusInternalServerError
	// Default response value is []byte("Server error")
	ServerErrResponse HTTPResponseConfig

	// NoRowsErrResponse is config used to respond to user if the returned
	// error result of AuthHandler#queryForUser is sql.ErrNoRows
	// This should be returned if there are no results when trying to grab
	// a user from the database
	//
	// Default status value is http.StatusInternalServerError
	// Default response value is []byte("User Not Found")
	//NoRowsErrResponse HTTPResponseConfig
}

type AuthHandler struct {
	db           httputil.DBInterfaceV2
	queryForUser QueryDB
	config       AuthHandlerConfig
}

func NewAuthHandler(
	db httputil.DBInterfaceV2,
	queryForUser QueryDB,
	config AuthHandlerConfig,
	//queryForSession func(http.ResponseWriter, httputil.DBInterfaceV2, string) (string, error),
	//sessionStore cacheutil.SessionStore,
) *AuthHandler {
	return &AuthHandler{
		//handler:      handler,
		// sessionStore: sessionStore,
		// sessionKeys:  sessionKeys,
		db:           db,
		queryForUser: queryForUser,
		config:       config,
		//queryForSession: queryForSession,
		//queryDB:      queryDB,
	}
}

func (a *AuthHandler) MiddlewareFunc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var userBytes []byte
		var middlewareUser middlewareUser
		var session *sessions.Session
		var err error

		// Setting up default values from passed configs if none are set
		setHTTPResponseDefaults(&a.config.DecodeCookieErrResponse, http.StatusBadRequest, []byte(invalidCookieTxt))
		setHTTPResponseDefaults(&a.config.ServerErrResponse, http.StatusInternalServerError, []byte(serverErrTxt))

		setUser := func() error {
			userBytes, err = a.queryForUser(w, r, a.db)

			if err != nil {
				isFatalErr := true

				switch err.(type) {
				case securecookie.Error:
					cookieErr := err.(securecookie.Error)

					if cookieErr.IsDecode() {
						isFatalErr = false
						w.WriteHeader(*a.config.DecodeCookieErrResponse.HTTPStatus)
						w.Write(a.config.DecodeCookieErrResponse.HTTPResponse)
					}

					w.WriteHeader(*a.config.ServerErrResponse.HTTPStatus)
					w.Write(a.config.ServerErrResponse.HTTPResponse)
				default:
					if err == sql.ErrNoRows {
						isFatalErr = false
						next.ServeHTTP(w, r)
						//return err
					} else {
						w.WriteHeader(*a.config.ServerErrResponse.HTTPStatus)
						w.Write(a.config.ServerErrResponse.HTTPResponse)
					}
				}

				if isFatalErr {
					httputil.Logger.Errorf("query for user err: %s", err.Error())
				}

				return err
			}

			err = json.Unmarshal(userBytes, &middlewareUser)

			if err != nil {
				w.WriteHeader(*a.config.ServerErrResponse.HTTPStatus)
				w.Write(a.config.ServerErrResponse.HTTPResponse)
				return err
			}

			return nil
		}

		// If user sets SessionStore, then we try retrieving session from implemented
		// SessionStore which usually is a file system or in-memory database i.e. Redis
		if a.config.SessionStore != nil {
			session, err = a.config.SessionStore.Get(r, a.config.SessionConfig.SessionName)

			if err != nil {
				w.WriteHeader(*a.config.ServerErrResponse.HTTPStatus)
				w.Write(a.config.ServerErrResponse.HTTPResponse)
				return
			}

			// If session is considered new, that means
			// either current user is truly not logged in or cache was/is down
			if session.IsNew {
				//fmt.Printf("new session\n")

				// First we determine if user is sending a cookie with our user cookie key
				// If they are, try retrieving from db if AuthHandler#queryForUser is set
				// Else, continue to next handler
				if _, err = r.Cookie(a.config.SessionConfig.SessionName); err == nil {
					//fmt.Printf("has cookie but not found in store\n")
					if err = setUser(); err != nil {
						fmt.Printf("within user\n")
						return
					}

					// Here we test to see if our session backend is responsive
					// If it is, that means current user logged in while cache was down
					// and was using the database to grab their sessions but since session
					// backend is back up, we can grab current user's session from
					// database and set it to session backend and use that instead of database
					// for future requests
					if _, err = a.config.SessionStore.Ping(); err == nil && a.config.QueryForSession != nil {
						//fmt.Printf("ping successful\n")
						sessionStr, err := a.config.QueryForSession(w, a.db, middlewareUser.ID)

						if err != nil {
							if err == sql.ErrNoRows {
								fmt.Printf("auth middleware db no row found\n")
								next.ServeHTTP(w, r)
								return
							}

							fmt.Printf("within query session\n")

							w.WriteHeader(*a.config.ServerErrResponse.HTTPStatus)
							w.Write(a.config.ServerErrResponse.HTTPResponse)
							return
						}

						fmt.Printf("session bytes: %s\n", sessionStr)

						session, err = a.config.SessionStore.New(r, a.config.SessionConfig.SessionName)

						if err != nil {
							fmt.Printf("within new session\n")
							w.WriteHeader(*a.config.ServerErrResponse.HTTPStatus)
							w.Write(a.config.ServerErrResponse.HTTPResponse)
							return
						}

						session.ID = sessionStr
						fmt.Printf("session id: %s\n", session.ID)
						session.Values[a.config.SessionConfig.Keys.UserKey] = userBytes
						session.Save(r, w)
					}

					//setCtxAndServe()
				} else {
					//fmt.Printf("new session, no cookie\n")
					next.ServeHTTP(w, r)
					return
				}
			} else {
				//fmt.Printf("not new session")
				if val, ok := session.Values[a.config.SessionConfig.Keys.UserKey]; ok {
					//fmt.Printf("found in session")
					userBytes = val.([]byte)
					err := json.Unmarshal(userBytes, &middlewareUser)

					if err != nil {
						httputil.Logger.Errorf("invalid json from session: %s", err.Error())
						w.WriteHeader(*a.config.ServerErrResponse.HTTPStatus)
						w.Write(a.config.ServerErrResponse.HTTPResponse)
						return
					}
				} else {
					next.ServeHTTP(w, r)
					return
				}
			}
		} else {
			if err = setUser(); err != nil {
				return
			}
		}

		ctx := context.WithValue(r.Context(), UserCtxKey, userBytes)
		ctxWithEmail := context.WithValue(ctx, MiddlewareUserCtxKey, middlewareUser)
		next.ServeHTTP(w, r.WithContext(ctxWithEmail))
	})
}

// setConfig is really only here for testing purposes
func (a *AuthHandler) setConfig(config AuthHandlerConfig) {
	a.config = config
}

// GroupHandlerConfig is config struct used for GroupHandler
// The settings don't have to be set but if programmer wants to
// be able to store user group information in cache instead
// of database, this can be achieved by implementing CacheStore
type GroupHandlerConfig struct {
	// CacheStore is used for retrieving results from a in-memory
	// database like Redis
	CacheStore cacheutil.CacheStore

	// IgnoreCacheNil will query database for group information
	// even if cache returns nil
	// CacheStore must be initialized to use this
	IgnoreCacheNil bool

	// ServerErrResponse is config used to respond to user if some type
	// of server error occurs
	//
	// Default status value is http.StatusInternalServerError
	// Default response value is []byte("Server error")
	ServerErrResponse HTTPResponseConfig
}

type GroupHandler struct {
	config         GroupHandlerConfig
	db             httputil.DBInterfaceV2
	queryForGroups QueryDB
}

func NewGroupHandler(
	db httputil.DBInterfaceV2,
	queryForGroups QueryDB,
	config GroupHandlerConfig,
) *GroupHandler {
	return &GroupHandler{
		config:         config,
		db:             db,
		queryForGroups: queryForGroups,
	}
}

func (g *GroupHandler) MiddlewareFunc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := r.Context().Value(MiddlewareUserCtxKey)

		if user != nil {
			var groupMap map[string]bool
			var err error
			var groupBytes []byte

			// Setting up default values from passed configs if none are set
			setHTTPResponseDefaults(&g.config.ServerErrResponse, http.StatusInternalServerError, []byte(serverErrTxt))
			user := user.(middlewareUser)
			groups := fmt.Sprintf(GroupKey, user.Email)

			setGroupFromDB := func() error {
				fmt.Printf("group middlware query db\n")
				groupBytes, err = g.queryForGroups(w, r, g.db)

				if err != nil {
					if err == sql.ErrNoRows {
						next.ServeHTTP(w, r)
						return err
					}

					w.WriteHeader(*g.config.ServerErrResponse.HTTPStatus)
					w.Write(g.config.ServerErrResponse.HTTPResponse)
					return err
				}

				err = json.Unmarshal(groupBytes, &groupMap)

				if err != nil {
					w.WriteHeader(*g.config.ServerErrResponse.HTTPStatus)
					w.Write(g.config.ServerErrResponse.HTTPResponse)
					return err
				}

				return nil
			}

			// If cache is set, try to get group info from cache
			if g.config.CacheStore != nil {
				groupBytes, err = g.config.CacheStore.Get(groups)

				if err != nil {
					// If err occurs and is not a nil err,
					// query from database
					if err != cacheutil.ErrCacheNil {
						if err = setGroupFromDB(); err != nil {
							return
						}
					} else {
						// If GroupHandlerConfig#IgnoreCacheNil is set,
						// then we ignore that the cache result came back
						// nil and query the database anyways
						if g.config.IgnoreCacheNil {
							if err = setGroupFromDB(); err != nil {
								return
							}
						} else {
							next.ServeHTTP(w, r)
							return
						}
					}
				}
			} else {
				if err = setGroupFromDB(); err != nil {
					return
				}
			}

			ctx := context.WithValue(r.Context(), GroupCtxKey, groupMap)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

type RoutingHandlerConfig struct {
	// CacheStore is used for retrieving results from a in-memory
	// database like Redis
	CacheStore cacheutil.CacheStore

	// IgnoreCacheNil will query database for group information
	// even if cache returns nil
	// CacheStore must be initialized to use this
	IgnoreCacheNil bool

	// ServerErrResponse is config used to respond to user if some type
	// of server error occurs
	//
	// Default status value is http.StatusInternalServerError
	// Default response value is []byte("Server Error")
	ServerErrResponse HTTPResponseConfig

	// UnauthorizedErrResponse is config used to respond to user if none
	// of the nonUserURLs keys or queried urls match the apis
	// a user is allowed to access
	//
	// Default status value is http.StatusForbidden
	// Default response value is []byte("Not authorized to access url")
	UnauthorizedErrResponse HTTPResponseConfig
}

type RoutingHandler struct {
	db          httputil.DBInterfaceV2
	queryDB     QueryDB
	pathRegex   httputil.PathRegex
	nonUserURLs map[string]bool
	config      RoutingHandlerConfig
}

func NewRoutingHandler(
	db httputil.DBInterfaceV2,
	queryDB QueryDB,
	pathRegex httputil.PathRegex,
	nonUserURLs map[string]bool,
	config RoutingHandlerConfig,
) *RoutingHandler {
	return &RoutingHandler{
		db:          db,
		queryDB:     queryDB,
		pathRegex:   pathRegex,
		nonUserURLs: nonUserURLs,
		config:      config,
	}
}

func (routing *RoutingHandler) MiddlewareFunc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//fmt.Printf("routing middleware\n")
		if r.Method != http.MethodOptions {
			var urlBytes []byte
			var urls map[string]bool
			var err error

			setHTTPResponseDefaults(&routing.config.UnauthorizedErrResponse, http.StatusInternalServerError, []byte(unauthorizedURLTxt))
			setHTTPResponseDefaults(&routing.config.ServerErrResponse, http.StatusInternalServerError, []byte(serverErrTxt))

			// Queries from db and sets the bytes returned to url map
			setURLsFromDB := func() error {
				urlBytes, err = routing.queryDB(w, r, routing.db)

				if err != nil {
					if err == sql.ErrNoRows {
						next.ServeHTTP(w, r)
						return err
					}

					w.WriteHeader(*routing.config.ServerErrResponse.HTTPStatus)
					w.Write(routing.config.ServerErrResponse.HTTPResponse)
					return err
				}

				err = json.Unmarshal(urlBytes, &urls)

				if err != nil {
					w.WriteHeader(*routing.config.ServerErrResponse.HTTPStatus)
					w.Write(routing.config.ServerErrResponse.HTTPResponse)
					return err
				}

				return nil
			}

			pathExp, err := routing.pathRegex(r)

			if err != nil {
				w.WriteHeader(*routing.config.ServerErrResponse.HTTPStatus)
				w.Write(routing.config.ServerErrResponse.HTTPResponse)
				return
			}

			allowedPath := false
			user := r.Context().Value(MiddlewareUserCtxKey)

			if user != nil {
				//fmt.Printf("routing user\n")
				user := user.(middlewareUser)
				key := fmt.Sprintf(URLKey, user.Email)

				if routing.config.CacheStore != nil {
					urlBytes, err = routing.config.CacheStore.Get(key)

					if err != nil {
						if err != cacheutil.ErrCacheNil {
							if err = setURLsFromDB(); err != nil {
								return
							}
						} else {
							// If RoutingHandlerConfig#IgnoreCacheNil is set,
							// then we ignore that the cache result came back
							// nil and query the database anyways
							if routing.config.IgnoreCacheNil {
								if err = setURLsFromDB(); err != nil {
									return
								}
							} else {
								next.ServeHTTP(w, r)
								return
							}
						}
					}

					//fmt.Printf("user path urls: %v\n", urls)
					if _, ok := urls[pathExp]; ok {
						allowedPath = true
					}
				}
			} else {
				//fmt.Printf("non user\n")
				//fmt.Printf("non user urls: %v\n", routing.nonUserURLs)
				if _, ok := routing.nonUserURLs[pathExp]; ok {
					allowedPath = true
				}
			}

			// If returned urls do not match an urls user is allowed to
			// access, return with error response
			if !allowedPath {
				w.WriteHeader(*routing.config.UnauthorizedErrResponse.HTTPStatus)
				w.Write(routing.config.UnauthorizedErrResponse.HTTPResponse)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
