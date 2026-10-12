package store

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDatabaseFilesArePrivateIncludingExistingDatabase(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	for _, existing := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "data", "soren.db")
		if existing {
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, nil, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, 0644); err != nil {
				t.Fatal(err)
			}
		}
		db, err := openSQLite(path)
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, _ := db.DB()
		if err := db.Exec("CREATE TABLE privacy_test (id INTEGER)").Error; err != nil {
			t.Fatal(err)
		}
		for _, suffix := range []string{"", "-wal", "-shm"} {
			info, err := os.Stat(path + suffix)
			if os.IsNotExist(err) && suffix != "" {
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm()&0077 != 0 {
				t.Fatalf("database%s permissions %o", suffix, info.Mode().Perm())
			}
		}
		_ = sqlDB.Close()
	}
}
