package packaging

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/mholt/archiver/v3"
	"github.com/stretchr/testify/assert"
)

const (
	testAgentFileName        = "agent"
	testAgentArchiveFileName = "agent.tar.gz"
	testDownloadDir          = "download"
)

func createTestArchive(t *testing.T, dir string) {
	filePath := path.Join(dir, testAgentFileName)
	err := os.WriteFile(filePath, []byte("test"), 0644)
	assert.NoError(t, err)
	archivePath := path.Join(dir, testAgentArchiveFileName)
	err = archiver.DefaultTarGz.Archive([]string{filePath}, archivePath)
	assert.NoError(t, err)
}

func createTestServer(t *testing.T, dir string) *httptest.Server {
	createTestArchive(t, dir)
	return httptest.NewServer(http.FileServer(http.Dir(dir)))
}

func agentArchiveHash(t *testing.T, dir string) []byte {
	f, err := os.Open(path.Join(dir, testAgentArchiveFileName))
	assert.NoError(t, err)
	defer f.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, f)
	assert.NoError(t, err)
	return hash.Sum(nil)
}

func TestDownload(t *testing.T) {
	dir := t.TempDir()
	server := createTestServer(t, dir)
	defer server.Close()
	downloader := NewDownloader(server.Client())
	downloadPath := path.Join(dir, testDownloadDir)
	err := os.MkdirAll(downloadPath, 0755)
	assert.NoError(t, err)

	err = downloader.Download(context.Background(), fmt.Sprintf("%s/%s", server.URL, testAgentArchiveFileName), agentArchiveHash(t, dir), downloadPath)
	assert.NoError(t, err)
	assert.FileExists(t, path.Join(downloadPath, testAgentFileName))
}

func TestDownloadCheckHash(t *testing.T) {
	dir := t.TempDir()
	server := createTestServer(t, dir)
	defer server.Close()
	downloader := NewDownloader(server.Client())
	downloadPath := path.Join(dir, testDownloadDir)
	err := os.MkdirAll(downloadPath, 0755)
	assert.NoError(t, err)

	fakeHash := sha256.Sum256([]byte(`test`))
	err = downloader.Download(context.Background(), fmt.Sprintf("%s/%s", server.URL, testAgentArchiveFileName), fakeHash[:], downloadPath)
	assert.Error(t, err)
}
