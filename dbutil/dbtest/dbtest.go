package dbtest

// --------------------------- TEST SUITES ------------------------------

type logTableReturn struct {
	ID        interface{}
	TimeStamp string
}

type PreTestConfig struct {
	LogTable     string
	TimeStampCol string
}

type PostTestConfig struct {
	LogTable     string
	DBTableCol   string
	DBIDCol      string
	TimeStampCol string
}

// func PreTest(db httputil.Querier, preTestConf *PreTestConfig) (string, error) {
// 	var timeStamp string
// 	scanner := db.QueryRow(`select ` + preTestConf.TimeStampCol + ` from ` + preTestConf.LogTable + ` order by desc` + preTestConf.TimeStampCol)
// 	err := scanner.Scan(&timeStamp)

// 	if err != nil {
// 		return "", err
// 	}

// 	return timeStamp, nil
// }

// func PostTest(db httputil.Querier, postTestConf *PostTestConfig, timeStamp string, updateQueries []string) error {
// 	var query string
// 	var err error

// 	if timeStamp == "" {
// 		query = `select ` + postTestConf.DBIDCol + `,` + postTestConf.DBTableCol + ` from ` + postTestConf.LogTable
// 	} else {
// 		query =
// 			`select ` +
// 				postTestConf.DBIDCol + `,` +
// 				postTestConf.DBTableCol +
// 				` from ` + postTestConf.LogTable +
// 				` where ` + postTestConf.TimeStampCol +
// 				` > ` + timeStamp
// 	}

// 	if updateQueries != nil {
// 		tx, _ := db.Begin()
// 		for _, v := range updateQueries {
// 			_, err = tx.Exec(v)

// 			if err != nil {
// 				return err
// 			}
// 		}

// 		err = tx.Commit()

// 		if err != nil {
// 			return err
// 		}
// 	}

// 	logReturns := make([]logTableReturn, 0)
// 	rower, err := db.Query(query)

// 	if err != nil {
// 		return err
// 	}

// 	tx, _ := db.Begin()
// 	for rower.Next() {
// 		var id interface{}
// 		var stamp string
// 		err = rower.Scan(&id, &stamp)

// 		if err != nil {
// 			return err
// 		}

// 		_, err = tx.Exec(`delete from` +)
// 	}

// 	err = tx.Commit()

// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }
