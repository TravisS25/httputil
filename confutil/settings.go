package confutil

type CacheOptions struct {
	Address  string
	Password string
	DB       string
}

type EmailConfig struct {
	TestMode  bool   `yaml:"test_mode"`
	TestEmail *Email `yaml:"test_email"`
	LiveEmail *Email `yaml:"live_email"`
}

type Email struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type RedisSession struct {
	Size       int    `yaml:"size"`
	Network    string `yaml:"network"`
	Address    string `yaml:"address"`
	Password   string `yaml:"password"`
	AuthKey    string `yaml:"auth_key"`
	EncryptKey string `yaml:"encrypt_key"`
}

type RedisCache struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type CookieStore struct {
	AuthKey    string `yaml:"auth_key"`
	EncryptKey string `yaml:"encrypt_key"`
}

type FileSystemStore struct {
	Dir        string `yaml:"dir"`
	AuthKey    string `yaml:"auth_key"`
	EncryptKey string `yaml:"encrypt_key"`
}

type StoreConfig struct {
	Redis           *RedisSession    `yaml:"redis"`
	FileSystemStore *FileSystemStore `yaml:"file_system_store"`
	CookieStore     *CookieStore     `yaml:"cookie_store"`
}

type CacheConfig struct {
	Redis *RedisCache
}

type Stripe struct {
	TestMode            bool   `yaml:"test_mode"`
	StripeTestSecretKey string `yaml:"stripe_test_secret_key"`
	StripeLiveSecretKey string `yaml:"stripe_live_secret_key"`
}

type DatabaseConfig struct {
	TestMode bool      `yaml:"test_mode"`
	Prod     *Database `yaml:"prod"`
	Test     *Database `yaml:"test"`
}

type Database struct {
	DBName   string `yaml:"db_name"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	SSlMode  string `yaml:"ssl_mode"`
}

// Settings is the configuration settings for the app
type Settings struct {
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
}
