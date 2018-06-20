package apiutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/sessions"

	"github.com/TravisS25/httputil/cacheutil"
	"github.com/gorilla/csrf"
	"github.com/gorilla/mux"
	"github.com/urfave/negroni"
)

const (
	GroupKey   = "%s-groups"
	URLKey     = "%s-urls"
	LockoutKey = "%s-lockout"
)

var (
	UserCtxKey  = Key{KeyName: "user"}
	GroupCtxKey = Key{KeyName: "groupName"}
)

type Key struct {
	KeyName string
}

type Middleware struct {
	cache           cacheutil.CacheStore
	sessionStore    sessions.Store
	user            IUser
	userSessionName string
	routing         map[string][]string
}

type IUser interface {
	ID() int
	FirstName() string
	LastName() string
	Email() string
	IsActive() bool
	DateJoined() string
}

func NewMiddleware(sessionStore sessions.Store, cache cacheutil.CacheStore, user IUser, routing map[string][]string) *Middleware {
	return &Middleware{
		cache:           cache,
		sessionStore:    sessionStore,
		user:            user,
		userSessionName: "user",
		routing:         routing,
	}
}

func (m *Middleware) SetUserSessionName(name string) {
	m.userSessionName = name
}

func (m *Middleware) CSRFTokenMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	w.Header().Set("X-CSRF-TOKEN", csrf.Token(r))
	next(w, r)
}

// AuthMiddleware is middleware used to check for authenication of incoming requests
func (m *Middleware) AuthMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	session, _ := m.sessionStore.Get(r, m.userSessionName)

	if _, ok := session.Values[m.userSessionName]; ok {
		var user interface{}
		err := json.Unmarshal(session.Values[m.userSessionName].([]byte), &user)

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

func OptionsMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	next(w, r)
}

// func LockoutMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
// 	user := GetUser(r)
// 	if user != nil {
// 		key := fmt.Sprintf(LockoutKey, user.Username)
// 		lockoutBytes, _ := config.Cache.Get(key)

// 		if lockoutBytes != nil && !strings.Contains(r.URL.Path, "/api/account/lockout") {
// 			fmt.Println("lockout redirect")
// 			w.WriteHeader(http.StatusSeeOther)
// 			return
// 		}
// 	}

// 	next(w, r)
// }

// RoutingMiddleware is middleware used to indicate whether an incoming request is
// authorized to go to certain urls based on authentication
// func (m *Middleware) RoutingMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
// 	rootPath := "/"
// 	path := r.URL.Path
// 	allowedPath := false
// 	userProfile := GetUser(r).(IUser)

// 	if r.Method != "OPTIONS" {
// 		if userProfile != nil {
// 			key := fmt.Sprintf(URLKey, userProfile.Email())
// 			urlBytes, err := m.cache.Get(key)

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
// 				for _, url := range confutil.Routing["Anon"] {
// 					if strings.Contains(path, url) && url != rootPath {
// 						allowedPath = true
// 						break
// 					}
// 				}
// 			}
// 		}

// 		if !allowedPath {
// 			fmt.Println("not allowed")
// 			fmt.Println(r.URL.Path)
// 			w.WriteHeader(http.StatusNotFound)
// 			return
// 		}

// 		next(w, r)
// 	} else {
// 		next(w, r)
// 	}
// }

func (m *Middleware) GroupMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	user := GetUser(r).(IUser)

	if user != nil {
		var groupArray []string
		groups := fmt.Sprintf(GroupKey, user.Email())
		groupBytes, _ := m.cache.Get(groups)
		json.Unmarshal(groupBytes, &groupArray)
		ctx := context.WithValue(r.Context(), GroupCtxKey, groupArray)
		next(w, r.WithContext(ctx))
	} else {
		next(w, r)
	}
}

func (m *Middleware) LoggingMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	res := w.(negroni.ResponseWriter)
	fmt.Println(r.Body)
	fmt.Println(res.Status())
	if r.Method == "PUT" {
		fmt.Println("within put")
	}

	if res.Status() == 0 {
		fmt.Println("status")
	}

	//user := GetUser(r)
	for _, v := range NonSafeOperations {
		fmt.Printf("method: %s", r.Method)
		fmt.Println("operation: %s", v)
		if (v == r.Method) && (res.Status() == 0 || res.Status() == 200) {
			fmt.Printf("request id: %s\n", mux.Vars(r))

			fmt.Println("helllllllo")
			//contents, _ := ioutil.ReadAll(r.Body)
			var item interface{}
			bodyBytes, _ := ioutil.ReadAll(r.Body)
			r.Body.Close()
			r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
			dec := json.NewDecoder(bytes.NewReader(bodyBytes))
			dec.Decode(&item)
			r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
			fmt.Println(item)
			// fmt.Println(item)
			// foo := "Old"
			// logging := models.Logging{
			// 	DateCreated:   time.Now().UTC().Format(confutil.DateTimeLayout),
			// 	Operation:     r.Method,
			// 	Old:           &foo,
			// 	New:           "New",
			// 	UserProfileID: &user.ID,
			// }
		}
	}

	next(w, r)
}
