//go:build windows

package microsoft

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// holdLikeExcel opens a file the way Excel holds a workbook it is editing: with write
// and DELETE access, sharing everything
func holdLikeExcel(t *testing.T, path string) {
	name, err := windows.UTF16PtrFromString(path)
	require.NoError(t, err)
	handle, err := windows.CreateFile(name,
		windows.GENERIC_READ|windows.GENERIC_WRITE|windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	require.NoError(t, err)
	t.Cleanup(func() { windows.CloseHandle(handle) })
}

func TestReadShared_WhileHeldLikeExcel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "held.xlsx")
	require.NoError(t, os.WriteFile(path, []byte("workbook bytes"), 0o644))
	holdLikeExcel(t, path)

	// this is the failure that made the operator close Excel before every run
	if _, err := os.ReadFile(path); err == nil {
		t.Log("os.ReadFile can now read a file held with DELETE access; Go may have started sharing DELETE")
	}

	data, err := readShared(path)
	require.NoError(t, err)
	assert.Equal(t, "workbook bytes", string(data))
}

func TestReadShared_GivesUpOnALockedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locked.xlsx")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0o644))

	// a handle that shares nothing at all can't be read past
	name, err := windows.UTF16PtrFromString(path)
	require.NoError(t, err)
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	require.NoError(t, err)
	defer windows.CloseHandle(handle)

	defer func(d time.Duration) { sharingRetryFor = d }(sharingRetryFor)
	sharingRetryFor = 300 * time.Millisecond

	start := time.Now()
	_, err = readShared(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locked.xlsx")
	assert.GreaterOrEqual(t, time.Since(start), 300*time.Millisecond, "it should have retried")
}
