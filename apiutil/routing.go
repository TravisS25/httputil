package apiutil

import (
	"github.com/gorilla/mux"
)

func SetRouterRegexPaths(router *mux.Router, paths map[string]string, routerRegexps map[string]string, routerRegexPaths map[string]string) {
	err := router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		exp, err := route.GetPathRegexp()

		if err != nil {
			return err
		}

		path, err := route.GetPathTemplate()

		if err != nil {
			return err
		}

		for k, v := range paths {
			if v == path {
				routerRegexps[exp] = path
				routerRegexPaths[k] = exp
				break
			}
		}

		return nil
	})

	if err != nil {
		panic(err.Error())
	}
}
