package confutil

// CacheOptions is config struct for cache settings
// type CacheOptions struct {
// 	Address  string
// 	Password string
// 	DB       string
// }

// EmailConfig is config struct for settings up different config
// email settings depending on test mode or not
type EmailConfig struct {
	TestMode  bool   `yaml:"test_mode"`
	TestEmail *Email `yaml:"test_email"`
	LiveEmail *Email `yaml:"live_email"`
}

// Email is config struct for email
type Email struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// RedisSession is config struct for setting up session store
// for redis server
type RedisSession struct {
	Size       int    `yaml:"size"`
	Network    string `yaml:"network"`
	Address    string `yaml:"address"`
	Password   string `yaml:"password"`
	AuthKey    string `yaml:"auth_key"`
	EncryptKey string `yaml:"encrypt_key"`
}

// RedisCache is config struct for setting up caching for
// a redis server
type RedisCache struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// CookieStore is config struct for storing sessions
// in cookies
type CookieStore struct {
	AuthKey    string `yaml:"auth_key"`
	EncryptKey string `yaml:"encrypt_key"`
}

// FileSystemStore is config struct for storing sessions
// in the file system
type FileSystemStore struct {
	Dir        string `yaml:"dir"`
	AuthKey    string `yaml:"auth_key"`
	EncryptKey string `yaml:"encrypt_key"`
}

// StoreConfig is overall config struct that allows user
// to easily configure all session store types
type StoreConfig struct {
	Redis           *RedisSession    `yaml:"redis"`
	FileSystemStore *FileSystemStore `yaml:"file_system_store"`
	CookieStore     *CookieStore     `yaml:"cookie_store"`
	AuthKey         string           `yaml:"auth_key"`
	EncryptKey      string           `yaml:"encrypt_key"`
}

type CacheConfig struct {
	Redis *RedisCache `yaml:"redis"`
}

// Stripe is config struct to set up stripe in app
type Stripe struct {
	TestMode            bool   `yaml:"test_mode"`
	StripeTestSecretKey string `yaml:"stripe_test_secret_key"`
	StripeLiveSecretKey string `yaml:"stripe_live_secret_key"`
}

// DatabaseConfig is overall config struct to set up
// multiple database configurations
type DatabaseConfig struct {
	TestMode bool      `yaml:"test_mode"`
	Prod     *Database `yaml:"prod"`
	Test     *Database `yaml:"test"`
}

// Database is config struct to set up database connection
type Database struct {
	DBName   string `yaml:"db_name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	SSLMode  string `yaml:"ssl_mode"`
}

type S3Config struct {
	IsProd  bool                  `yaml:"is_prod"`
	Buckets map[string]*S3Storage `yaml:"buckets"`
}

type S3Storage struct {
	EndPoint        string `json:"end_point"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	UseSSL          bool   `yaml:"use_ssl"`
}

// Settings is the configuration settings for the app
type Settings struct {
	Prod bool `yaml:"prod"`
	// AuthKey        string          `yaml:"auth_key"`
	// EncryptKey     string          `yaml:"encrypt_key"`
	Domain         string          `yaml:"domain"`
	ClientDomain   string          `yaml:"client_domain"`
	CSRF           string          `yaml:"csrf"`
	TemplatesDir   string          `yaml:"templates_dir"`
	HTTPS          bool            `yaml:"https"`
	AssetsLocation string          `yaml:"assets_location"`
	AllowedOrigins []string        `yaml:"allowed_origins"`
	EmailConfig    *EmailConfig    `yaml:"email_config"`
	Store          *StoreConfig    `yaml:"store"`
	Cache          *CacheConfig    `yaml:"cache"`
	DatabaseConfig *DatabaseConfig `yaml:"database_config"`
	Stripe         *Stripe         `yaml:"stripe"`
	S3Config       *S3Config       `yaml:"s3_config"`

	Databases map[string][]*Database `yaml:"databases"`
	Emails    map[string]*Email      `yaml:"emails"`
	StripeMap map[string]*Stripe     `yaml:"stripe_map"`
}
