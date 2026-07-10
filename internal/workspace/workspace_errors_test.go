package workspace_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManager_Write_resolveError(t *testing.T) {
	m := newTestManager(t)
	err := m.Write("../outside.txt", []byte("x"))
	require.Error(t, err)
}

func TestManager_MkdirAll_resolveError(t *testing.T) {
	m := newTestManager(t)
	err := m.MkdirAll("../outside")
	require.Error(t, err)
}

func TestManager_List_resolveError(t *testing.T) {
	m := newTestManager(t)
	_, err := m.List("../outside")
	require.Error(t, err)
}

func TestManager_Write_parentIsFile(t *testing.T) {
	m := newTestManager(t)
	require.NoError(t, m.Write("parent.txt", []byte("file")))
	err := m.Write("parent.txt/child.txt", []byte("x"))
	require.Error(t, err)
}
