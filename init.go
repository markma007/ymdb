package ymdb

import (
	"errors"
	"fmt"
	"sync"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// DBManager owns a database connection. Prefer its methods in applications;
// the package-level DB and DBM variables remain for backwards compatibility.
type DBManager struct {
	DB *gorm.DB
}

var (
	DBM  *DBManager
	DB   *gorm.DB
	dbMu sync.RWMutex
)

// Open opens and migrates a database without changing package-global state.
func Open(dbFilepath string) (*DBManager, error) {
	if dbFilepath == "" {
		return nil, errors.New("ymdb: database filepath is required")
	}
	db, err := gorm.Open(sqlite.Open(dbFilepath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("ymdb: open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("ymdb: access connection pool: %w", err)
	}
	// SQLite pragmas apply per connection. A single writer connection makes
	// foreign keys reliable and avoids in-process SQLITE_BUSY write races.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	for _, pragma := range []string{"PRAGMA foreign_keys = ON", "PRAGMA journal_mode = WAL", "PRAGMA busy_timeout = 5000"} {
		if err := db.Exec(pragma).Error; err != nil {
			_ = sqlDB.Close()
			return nil, fmt.Errorf("ymdb: configure sqlite (%s): %w", pragma, err)
		}
	}
	if err := db.AutoMigrate(
		&Post{}, &PostMeta{},
		&OptionModel{}, &OptionMeta{},
		&User{}, &UserMeta{},
	); err != nil {
		return nil, fmt.Errorf("ymdb: migrate database: %w", err)
	}
	if err := InstallDefaultFixtures(db); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ymdb: install default fixtures: %w", err)
	}
	return &DBManager{DB: db}, nil
}

// InitiateDBM initializes the legacy package-global manager.
func InitiateDBM(dbFilepath string) error {
	m, err := Open(dbFilepath)
	if err != nil {
		return err
	}
	return m.Activate()
}

// Activate makes this manager the database used by package-level model APIs.
func (dbm *DBManager) Activate() error {
	if dbm == nil || dbm.DB == nil {
		return errors.New("ymdb: cannot activate a nil database manager")
	}
	dbMu.Lock()
	defer dbMu.Unlock()
	old := DBM
	DBM, DB = dbm, dbm.DB
	if old != nil && old != dbm {
		_ = old.close()
	}
	return nil
}

// Reconnect replaces this manager's connection after a successful open.
func (dbm *DBManager) Reconnect(dbFilepath string) error {
	m, err := Open(dbFilepath)
	if err != nil {
		return err
	}
	dbMu.Lock()
	old := dbm.DB
	dbm.DB = m.DB
	if DBM == dbm {
		DB = m.DB
	}
	dbMu.Unlock()
	if old != nil {
		if sqlDB, e := old.DB(); e == nil {
			_ = sqlDB.Close()
		}
	}
	return nil
}

// Close releases the underlying SQL connection pool.
func (dbm *DBManager) Close() error {
	if dbm == nil {
		return nil
	}
	dbMu.Lock()
	if DBM == dbm {
		DBM, DB = nil, nil
	}
	dbMu.Unlock()
	return dbm.close()
}

func (dbm *DBManager) close() error {
	if dbm == nil || dbm.DB == nil {
		return nil
	}
	sqlDB, err := dbm.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func defaultDB() (*gorm.DB, error) {
	dbMu.RLock()
	defer dbMu.RUnlock()
	if DB == nil {
		return nil, errors.New("ymdb: database is not initialized; call InitiateDBM or Open")
	}
	return DB, nil
}
