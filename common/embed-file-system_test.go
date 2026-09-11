package common

import (
	"embed"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/embed-assets/index.html
var testEmbeddedAssets embed.FS

func TestEmbedFolderLetsRootFallThroughToRuntimeRenderer(t *testing.T) {
	assets := EmbedFolder(testEmbeddedAssets, "testdata/embed-assets")

	assert.False(t, assets.Exists("/", "/"))
	assert.True(t, assets.Exists("/", "/index.html"))

	indexPage, err := assets.Open("/index.html")
	require.NoError(t, err)
	require.NoError(t, indexPage.Close())

	_, err = assets.Open("/")
	require.ErrorIs(t, err, os.ErrNotExist)
}
