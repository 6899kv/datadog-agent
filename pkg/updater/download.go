// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	agentArchiveFileName = "agent.tar.gz"
)

// downloader is the downloader used by the updater to download packages.
type downloader struct {
	client *http.Client
}

// newDownloader returns a new Downloader.
func newDownloader(client *http.Client) *downloader {
	return &downloader{
		client: client,
	}
}

// Download downloads the package at the given URL in temporary directory,
// verifies its SHA256 hash and extracts it to the given destination path.
// It currently assumes the package is a tar.gz archive.
func (d *downloader) Download(ctx context.Context, pkg Package, destinationPath string) error {
	tmpDir, err := os.MkdirTemp("", "")
	if err != nil {
		return fmt.Errorf("could not create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pkg.URL, nil)
	if err != nil {
		return fmt.Errorf("could not create download request: %w", err)
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("could not download package: %w", err)
	}
	defer resp.Body.Close()
	hashWriter := sha256.New()
	reader := io.TeeReader(resp.Body, hashWriter)
	archivePath := filepath.Join(tmpDir, agentArchiveFileName)
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("could not create archive file: %w", err)
	}
	defer archiveFile.Close()
	_, err = io.Copy(archiveFile, reader)
	if err != nil {
		return fmt.Errorf("could not write archive file: %w", err)
	}
	sha256 := hashWriter.Sum(nil)
	expectedHash, err := hex.DecodeString(pkg.SHA256)
	if err != nil {
		return fmt.Errorf("could not decode hash: %w", err)
	}
	if !bytes.Equal(expectedHash, sha256) {
		return fmt.Errorf("invalid hash for %s: expected %x, got %x", pkg.URL, pkg.SHA256, sha256)
	}
	err = extractTarGz(archivePath, destinationPath)
	if err != nil {
		return fmt.Errorf("could not extract archive: %w", err)
	}
	return nil
}

func extractTarGz(archivePath string, destinationPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("could not open archive: %w", err)
	}
	defer f.Close()
	gzr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("could not create gzip reader: %w", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("could not read tar header: %w", err)
		}
		target := filepath.Join(destinationPath, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, 0755)
			if err != nil {
				return fmt.Errorf("could not create directory: %w", err)
			}
		case tar.TypeReg:
			err := os.MkdirAll(filepath.Dir(target), 0755)
			if err != nil {
				return fmt.Errorf("could not create directory: %w", err)
			}
			f, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("could not create file: %w", err)
			}
			defer f.Close()
			_, err = io.Copy(f, tr)
			if err != nil {
				return fmt.Errorf("could not write file: %w", err)
			}
		}
	}
	return nil
}