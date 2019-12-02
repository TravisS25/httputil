package apiutil

// type MockDB struct {
// 	QueryRowFunc func(query string, args ...interface{}) httputil.Scanner
// 	QueryFunc    func(query string, args ...interface{}) (httputil.Rower, error)
// 	ExecFunc     func(string, ...interface{}) (sql.Result, error)

// 	BeginFunc  func() (tx httputil.Tx, err error)
// 	CommitFunc func(tx httputil.Tx) error

// 	GetFunc    func(dest interface{}, query string, args ...interface{}) error
// 	SelectFunc func(dest interface{}, query string, args ...interface{}) error

// 	RecoverErrorFunc func(err error) bool
// }

const (
	statusErrTxt = "Status should be %d; got %d"
)
