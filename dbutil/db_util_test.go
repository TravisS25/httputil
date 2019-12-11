package dbutil

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/TravisS25/httputil"
	"github.com/TravisS25/httputil/confutil"
	"github.com/gorilla/mux"
)

var (
	dbMutex sync.Mutex
)

type Message struct {
	ready chan struct{}
}

func NewMessage() *Message {
	return &Message{ready: make(chan struct{})}
}

func (m *Message) Stop() {
	close(m.ready)
}

func (m *Message) Get() <-chan struct{} {
	return m.ready
}

func TestFoobar(t *testing.T) {
	var wg sync.WaitGroup

	oneShot := NewMessage()
	r := mux.NewRouter()
	r.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		fmt.Printf("request from %s\n", req.RemoteAddr)
	})

	numOfClients := 4
	s := httptest.NewServer(r)

	for i := 0; i < numOfClients; i++ {
		wg.Add(1)
		c := s.Client()
		go func(i int) {
			for {
				time.Sleep(time.Millisecond * 500)
				_, err := c.Get(s.URL + "/test")

				if err != nil {
					t.Errorf("%s\n", err.Error())
				}

				select {
				case <-oneShot.Get():
					fmt.Printf("stop request from client\n")
					wg.Done()
					return
				default:
					fmt.Printf("default\n")
					continue
					// wg.Done()
					// break L
				}
			}
		}(i)
	}
	time.Sleep(time.Second)
	oneShot.Stop()
	wg.Wait()

	t.Fatalf("boom")
}

func TestRecoveryErrorIntegrationTest(t *testing.T) {
	var err error

	dbList := []confutil.Database{
		{
			DBName:   "test1",
			User:     "test",
			Password: "password",
			Host:     "localhost",
			Port:     "26257",
			SSLMode:  SSLRequire,
		},
		{
			DBName:   "test2",
			User:     "test",
			Password: "password",
			Host:     "localhost",
			Port:     "26258",
			SSLMode:  SSLRequire,
		},
		{
			DBName:   "test3",
			User:     "test",
			Password: "password",
			Host:     "localhost",
			Port:     "26259",
			SSLMode:  SSLRequire,
		},
	}

	var db httputil.DBInterfaceV2
	db, err = NewDBWithList(dbList, Postgres)

	if err != nil {
		t.Fatalf(err.Error())
	}

	var wg sync.WaitGroup

	RecoverFromError := func(err error) error {
		dbMutex.Lock()
		defer dbMutex.Unlock()
		db, err = db.RecoverError(err)
		return err
	}

	oneShot := NewMessage()
	r := mux.NewRouter()
	r.HandleFunc("/test", func(w http.ResponseWriter, req *http.Request) {
		fmt.Printf("req from: %s\n", req.RemoteAddr)

		scanner := db.QueryRow(
			`
			select 
				crdb_internal.zones.zone_name
			from	
				crdb_internal.zones
			where
				crdb_internal.zones.zone_name = '.default'
			`,
		)

		var name string
		err = scanner.Scan(&name)

		if err != nil {
			fmt.Printf("db is down from req: %s\n", req.RemoteAddr)

			if err = RecoverFromError(err); err == nil {
				fmt.Printf("able to recover err from req: %s\n", req.RemoteAddr)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Printf("could not recover err from req: %s\n", req.RemoteAddr)
			}

		} else {
			fmt.Printf("No db error from reg: %s\n", req.RemoteAddr)
		}
	})

	numOfClients := 50
	s := httptest.NewServer(r)

	// Spin up threads of clients making requests to test api point
	for i := 0; i < numOfClients; i++ {
		wg.Add(1)
		c := s.Client()
		go func() {
			for {
				time.Sleep(time.Millisecond * 700)
				res, err := c.Get(s.URL + "/test")

				if err != nil {
					t.Errorf("%s\n", err.Error())
				}

				if res.StatusCode != http.StatusOK {
					t.Errorf("Did not return staus ok\n")
				}

				select {
				case <-oneShot.Get():
					fmt.Printf("stop request from client\n")
					wg.Done()
					return
				default:
					fmt.Printf("default foo\n")
					continue
					// wg.Done()
					// break L
				}
			}
		}()
	}

	// Allow for the clients to make a couple of requests
	time.Sleep(time.Second * 2)

	cmd := exec.Command("cockroach", "quit", "--host", dbList[0].Host+":"+dbList[0].Port)
	err = cmd.Start()

	if err != nil {
		t.Fatalf("Could not quit database\n")
	}

	// Allow for at least one client to connect to
	// db while down to try to recover and allow other
	// clients to connect to new db connection
	time.Sleep(time.Second * 5)

	oneShot.Stop()
	wg.Wait()

	// Bring cockroachdb back online
	h := os.Getenv("HOME")
	cmd = exec.Command(
		"cockroach",
		"start",
		"--certs-dir="+h+"/.cockroach-certs",
		"--store="+h+"/store1",
		"--listen-addr="+dbList[0].Host+":"+dbList[0].Port,
		"--http-addr=localhost:8080",
		"--join=localhost",
		"--background",
	)
	err = cmd.Start()

	if err != nil {
		t.Fatalf("Could not bring database back up\n")
	}

	//t.Fatalf("boom")
}

func TestFob(t *testing.T) {
	var err error

	conf := confutil.Database{
		DBName:   "test1",
		User:     "test",
		Password: "password",
		Host:     "127.0.0.1",
		Port:     "26257",
		SSLMode:  SSLRequire,
	}

	db, _ := NewDB(conf, Postgres)

	if err != nil {
		t.Fatalf("err: %s\n", err.Error())
	}

	cmd := exec.Command("cockroach", "quit", "--host", conf.Host+":"+conf.Port)
	err = cmd.Start()

	if err != nil {
		t.Fatalf("Could not quit database\n")
	}

	time.Sleep(time.Second * 6)

	dbInfo := fmt.Sprintf(
		DBConnStr,
		conf.Host,
		conf.User,
		conf.Password,
		conf.DBName,
		conf.Port,
		conf.SSLMode,
	)

	_, err = db.Driver().Open(dbInfo)

	if err != nil {
		t.Fatalf("err: %s\n", err.Error())
	}

	// scanner := db.QueryRow(
	// 	`
	// select
	// 	crdb_internal.zones.zone_name
	// from
	// 	crdb_internal.zones
	// where
	// 	crdb_internal.zones.zone_name = '.default'
	// `,
	// )

	// var name string
	// err = scanner.Scan(&name)

	// if err != nil {
	// 	t.Fatalf("err: %s\n", err.Error())
	// }

	// err := db.Ping()

	// if err != nil {
	// 	t.Fatalf("err: %s\n", err.Error())
	// }
}

func TestBlah(t *testing.T) {
	h := os.Getenv("HOME")

	t.Errorf(h)
}
