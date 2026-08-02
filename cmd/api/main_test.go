package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicOriginFromEnv(t *testing.T) {
	const name = "TEST_PUBLIC_ORIGIN"
	t.Cleanup(func() { _ = os.Unsetenv(name) })

	t.Run("normalizes trailing slash", func(t *testing.T) {
		require.NoError(t, os.Setenv(name, "https://ARCHIVER.example.com:443/"))
		origin, err := publicOriginFromEnv(name)
		require.NoError(t, err)
		assert.Equal(t, "https://archiver.example.com", origin)
	})

	for _, value := range []string{"", "archiver.example.com", "ftp://archiver.example.com", "https://user@archiver.example.com", "https://archiver.example.com/path"} {
		t.Run("rejects "+value, func(t *testing.T) {
			require.NoError(t, os.Setenv(name, value))
			_, err := publicOriginFromEnv(name)
			assert.Error(t, err)
		})
	}
}
