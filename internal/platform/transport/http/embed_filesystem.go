package http

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-contrib/static"
)

type embedFileSystem struct {
	http.FileSystem
}

func (e *embedFileSystem) Exists(prefix string, path string) bool {
	_, err := e.Open(path)
	return err == nil
}

func (e *embedFileSystem) Open(name string) (http.File, error) {
	if name == "/" {
		return nil, os.ErrNotExist
	}

	cleanName := path.Clean(strings.TrimPrefix(name, "/"))
	if cleanName == "." || cleanName == "" {
		return nil, os.ErrNotExist
	}

	file, err := e.FileSystem.Open(cleanName)
	if err == nil {
		return file, nil
	}

	return e.FileSystem.Open(path.Join(cleanName, "index.html"))
}

func embedFolder(fsEmbed embed.FS, targetPath string) static.ServeFileSystem {
	efs, err := fs.Sub(fsEmbed, targetPath)
	if err != nil {
		panic(err)
	}
	return &embedFileSystem{FileSystem: http.FS(efs)}
}
