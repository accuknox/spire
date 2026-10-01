//go:build cgo
// +build cgo

package sqlstore

import (
	"errors"

	"github.com/jinzhu/gorm"
	"github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"

	// gorm sqlite dialect init registration
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type sqliteDB struct {
	log logrus.FieldLogger
}

func (s sqliteDB) connect(cfg *configuration, isReadOnly bool) (db *gorm.DB, version string, supportsCTE bool, err error) {
	if isReadOnly {
		s.log.Warn("Read-only connection is not applicable for sqlite3. Falling back to primary connection")
	}

	db, err = openSQLite3(cfg.ConnectionString)
	if err != nil {
		return nil, "", false, err
	}

	version, err = queryVersion(db, "SELECT sqlite_version()")
	if err != nil {
		return nil, "", false, err
	}

	// The embedded version of SQLite3 unconditionally supports CTE.
	return db, version, true, nil
}

func (s sqliteDB) isConstraintViolation(err error) bool {
	if err == nil {
		return false
	}
	var e sqlite3.Error
	ok := errors.As(err, &e)
	return ok && e.Code == sqlite3.ErrConstraint
}

func openSQLite3(connString string) (*gorm.DB, error) {
	embellished, err := embellishSQLite3ConnString(connString)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open("sqlite3", embellished)
	if err != nil {
		return nil, sqlError.Wrap(err)
	}
	return db, nil
}
