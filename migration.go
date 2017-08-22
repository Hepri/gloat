package gloat

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	now = time.Now().UTC()

	nameNormalizerRe = regexp.MustCompile(`([a-z])([A-Z])`)
	versionFormat    = "20060102150405"
)

// Migration holds all the relevant information for a migration. The content of
// the UP side, the DOWN side, a path and version. The version is used to
// determine the order of which the migrations would be executed. The path is
// the name in a store.
type Migration struct {
	UpSQL   []byte
	DownSQL []byte
	Path    string
	Version int64
}

// Reversible returns true if the migration DownSQL content is present. E.g. if
// both of the directions are present in the migration folder.
func (m *Migration) Reversible() bool {
	return len(m.DownSQL) != 0
}

// Persistable is any migration with non blank Path.
func (m *Migration) Persistable() bool {
	return m.Path != ""
}

// GenerateMigration generates a new blank migration with blank UP and DOWN
// content defined from user entered content.
func GenerateMigration(str string) *Migration {
	version := generateVersion()
	path := generateMigrationPath(version, str)

	return &Migration{
		Path:    path,
		Version: version,
	}
}

// MigrationFromBytes builds a Migration struct from a path and a
// function. Functions like ioutil.ReadFile, go-bindata's Asset have
// the very same signature, so you can use them here.
func MigrationFromBytes(path string, fn func(string) ([]byte, error)) (*Migration, error) {
	version, err := versionFromPath(path)
	if err != nil {
		return nil, err
	}

	upSQL, err := fn(filepath.Join(path, "up.sql"))
	if err != nil {
		return nil, err
	}

	// This may be an error from the OS or the go-bindata generated Asset
	// function ("Asset %s can't read by error: %v"). Just ignore it, as we can
	// have embedded irreversible migrations.
	downSQL, _ := fn(filepath.Join(path, "down.sql"))

	return &Migration{
		UpSQL:   upSQL,
		DownSQL: downSQL,
		Path:    path,
		Version: version,
	}, nil
}

func generateMigrationPath(version int64, str string) string {
	name := strings.ToLower(nameNormalizerRe.ReplaceAllString(str, "${1}_${2}"))
	return fmt.Sprintf("%d_%s", version, name)
}

func generateVersion() int64 {
	version, _ := strconv.ParseInt(now.Format(versionFormat), 10, 64)
	return version
}

func versionFromPath(path string) (int64, error) {
	parts := strings.SplitN(filepath.Base(path), "_", 2)
	if len(parts) == 0 {
		return 0, fmt.Errorf("cannot extract version from %s", path)
	}

	return strconv.ParseInt(parts[0], 10, 64)
}

// Migrations is a slice of Migration pointers.
type Migrations []*Migration

// Except selects migrations that does not exist in the current ones.
func (m Migrations) Except(migrations Migrations) (excepted Migrations) {
	current := map[int64]bool{}
	for _, migration := range m {
		current[migration.Version] = true
	}

	new := map[int64]bool{}
	for _, migration := range migrations {
		new[migration.Version] = true
	}

	for _, migration := range m {
		if !new[migration.Version] {
			excepted = append(excepted, migration)
		}
	}

	for _, migration := range migrations {
		if !current[migration.Version] {
			excepted = append(excepted, migration)
		}
	}

	return
}

// Implementation for the sort.Sort interface.

func (m Migrations) Len() int           { return len(m) }
func (m Migrations) Less(i, j int) bool { return m[i].Version < m[j].Version }
func (m Migrations) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

// Sort is a convenience sorting method.
func (m Migrations) Sort() { sort.Sort(m) }

// Current returns the latest applied migration. Can be nil, if the migrations
// are empty.
func (m Migrations) Current() *Migration {
	m.Sort()

	if len(m) == 0 {
		return nil
	}

	return m[len(m)-1]
}

// UnappliedMigrations selects the unapplied migrations from a Source. For a
// migration to be unapplied it should not be present in the Store.
func UnappliedMigrations(source Source, store Store) (Migrations, error) {
	allMigrations, err := source.Collect()
	if err != nil {
		return nil, err
	}

	appliedMigrations, err := store.Collect()
	if err != nil {
		return nil, err
	}

	unappliedMigrations := allMigrations.Except(appliedMigrations)
	unappliedMigrations.Sort()

	return unappliedMigrations, nil
}
