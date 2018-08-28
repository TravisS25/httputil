package startutil

import (
	"html/template"
	"net/http"

	"github.com/TravisS25/httputil/formutil"

	"github.com/TravisS25/httputil"
	"github.com/TravisS25/httputil/cacheutil"
	"github.com/TravisS25/httputil/confutil"
	"github.com/TravisS25/httputil/dbutil"
	"github.com/TravisS25/httputil/mailutil"
	"github.com/go-redis/redis"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	redistore "gopkg.in/boj/redistore.v1"
)

func SetFormValidator(formValidation *formutil.FormValidation, db httputil.Querier, cache cacheutil.CacheStore) {
	formValidation.SetQuerier(db)
	formValidation.SetCache(cache)
}

func SetConfigSettings(conf *confutil.Settings, envVar string) {
	conf = confutil.ConfigSettings(envVar)
}

func SetCacheSettings(conf *confutil.Settings, cache cacheutil.CacheStore) {
	if conf.Cache.Redis != nil {
		redisClient := redis.NewClient(&redis.Options{
			Addr:     conf.Cache.Redis.Address,
			Password: conf.Cache.Redis.Password,
			DB:       conf.Cache.Redis.DB,
		})
		cache = cacheutil.NewClientCache(redisClient)
	}
}

func SetDB(conf *confutil.Settings, isProd bool, db *dbutil.DB) {
	var err error

	if isProd {
		db, err = dbutil.NewDB(dbutil.DBConfig{
			Host:     conf.DatabaseConfig.Prod.Host,
			User:     conf.DatabaseConfig.Prod.User,
			Password: conf.DatabaseConfig.Prod.Password,
			DBName:   conf.DatabaseConfig.Prod.DBName,
			Port:     conf.DatabaseConfig.Prod.Port,
			SSLMode:  conf.DatabaseConfig.Prod.SSlMode,
		})
	} else {
		db, err = dbutil.NewDB(dbutil.DBConfig{
			Host:     conf.DatabaseConfig.Test.Host,
			User:     conf.DatabaseConfig.Test.User,
			Password: conf.DatabaseConfig.Test.Password,
			DBName:   conf.DatabaseConfig.Test.DBName,
			Port:     conf.DatabaseConfig.Test.Port,
			SSLMode:  conf.DatabaseConfig.Test.SSlMode,
		})
	}

	if err != nil {
		panic(err)
	}
}

func SetStoreSettings(conf *confutil.Settings, store sessions.Store) {
	var err error

	if conf.Store.Redis != nil {
		store, err = redistore.NewRediStore(
			conf.Store.Redis.Size,
			conf.Store.Redis.Network,
			conf.Store.Redis.Address,
			conf.Store.Redis.Password,
			[]byte(conf.Store.Redis.AuthKey),
			[]byte(conf.Store.Redis.EncryptKey),
		)
	} else if conf.Store.FileSystemStore != nil {
		store = sessions.NewFilesystemStore(
			"/tmp",
			[]byte(conf.Store.FileSystemStore.AuthKey),
			[]byte(conf.Store.FileSystemStore.EncryptKey),
		)
	} else {
		store = sessions.NewCookieStore(
			[]byte(conf.Store.CookieStore.AuthKey),
			[]byte(conf.Store.CookieStore.EncryptKey),
		)
	}

	if err != nil {
		panic(err)
	}
}

func SetMessenger(conf *confutil.Settings, mailer mailutil.SendMessage) {
	if conf.EmailConfig.TestMode {
		mailer = mailutil.NewMailMessenger(mailutil.MailerConfig{
			Host:     conf.EmailConfig.TestEmail.Host,
			Port:     conf.EmailConfig.TestEmail.Port,
			User:     conf.EmailConfig.TestEmail.User,
			Password: conf.EmailConfig.TestEmail.Password,
		})
	} else {
		mailer = mailutil.NewMailMessenger(mailutil.MailerConfig{
			Host:     conf.EmailConfig.LiveEmail.Host,
			Port:     conf.EmailConfig.LiveEmail.Port,
			User:     conf.EmailConfig.LiveEmail.User,
			Password: conf.EmailConfig.LiveEmail.Password,
		})
	}
}

func SetTemplate(conf *confutil.Settings, templ *template.Template) {
	templ = template.Must(template.ParseGlob(conf.TemplatesDir))
}

func SetCSRF(conf *confutil.Settings, token func(http.Handler) http.Handler, cookieName string) {
	token = csrf.Protect([]byte(conf.CSRF), csrf.Secure(conf.HTTPS), csrf.CookieName(cookieName))
}
