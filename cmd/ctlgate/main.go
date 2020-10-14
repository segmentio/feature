package main

import (
	"database/sql"
	"fmt"

	sqlite3 "github.com/mattn/go-sqlite3"
	"github.com/segmentio/cli"
)

func init() {
	sql.Register("ctlgate", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			// This turns off automatic WAL checkpoints in the reader. Since the reader
			// can't do checkpoints as it's usually in read-only mode, checkpoints only
			// result in an error getting returned to callers in some circumstances.
			// As the Reflector is the only writer to the LDB, and it will continue to
			// run checkpoints, the WAL will stay nice and tidy.
			_, err := conn.Exec("PRAGMA wal_autocheckpoint = 0", nil)
			return err
		},
	})
}

func main() {
	cli.Exec(cli.CommandSet{
		"sync": cli.Command(sync),
	})
}

func openDB(path string) (*sql.DB, error) {
	return sql.Open("ctlgate",
		fmt.Sprintf("file:%s?_journal_mode=wal&mode=rwc", path))
}

func withDB(path string, do func(*sql.DB) error) error {
	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close()
	return do(db)
}
