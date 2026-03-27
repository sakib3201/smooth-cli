package store_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/smoothcli/smooth-cli/internal/store"
)

func TestSQLite_Migration_RunsOnFirstOpen(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db, err := store.OpenDB(dbPath)
	require.NoError(t, err)
	defer db.Close()

	var version int
	row := db.QueryRow("SELECT version FROM schema_version LIMIT 1")
	err = row.Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 1, version)
}

func TestSQLite_Migration_IsIdempotent(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db1, err := store.OpenDB(dbPath)
	require.NoError(t, err)
	db1.Close()

	db2, err := store.OpenDB(dbPath)
	require.NoError(t, err)
	defer db2.Close()

	var version int
	row := db2.QueryRow("SELECT version FROM schema_version LIMIT 1")
	err = row.Scan(&version)
	require.NoError(t, err)
	assert.Equal(t, 1, version)
}

func TestSQLite_PermissionRequest_PersistedAndRecovered(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db1, err := store.OpenDB(dbPath)
	require.NoError(t, err)

	s := store.New(db1)
	s.AddPermissionRequest(validPermissionReq())

	err = s.Close()
	require.NoError(t, err)

	db2, err := store.OpenDB(dbPath)
	require.NoError(t, err)
	defer db2.Close()

	s2 := store.New(db2)
	pending := s2.PendingPermissionRequests()
	require.Len(t, pending, 1)
	assert.Equal(t, validPermissionReq().ID, pending[0].ID)
}

func TestSQLite_AttentionEvent_PersistedAndRecovered(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db1, err := store.OpenDB(dbPath)
	require.NoError(t, err)

	s := store.New(db1)
	s.AddAttentionEvent(validAttentionEvent())

	err = s.Close()
	require.NoError(t, err)

	db2, err := store.OpenDB(dbPath)
	require.NoError(t, err)
	defer db2.Close()

	s2 := store.New(db2)
	events := s2.ActiveAttentionEvents()
	require.Len(t, events, 1)
}

func TestSQLite_WAL_Mode_EnabledOnOpen(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")

	db, err := store.OpenDB(dbPath)
	require.NoError(t, err)
	defer db.Close()

	var mode string
	row := db.QueryRow("PRAGMA journal_mode")
	err = row.Scan(&mode)
	require.NoError(t, err)
	assert.Equal(t, "wal", mode)
}
