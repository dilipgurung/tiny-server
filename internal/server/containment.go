package server

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// containedFS wraps an *os.Root filesystem so paths the root refuses to
// resolve are reported to net/http as "not exist".
//
// os.Root rejects any path that resolves outside the served directory --
// most importantly a symlink such as "config -> ../../.env" -- but it does
// so with an unexported "path escapes from parent" error. http.FileServerFS
// only recognises fs.ErrNotExist and fs.ErrPermission, so anything else
// surfaces to the client as 500 Internal Server Error, which both looks
// broken and reveals that the entry exists. A path the server will never
// serve is indistinguishable from a missing one as far as a client is
// concerned, so report it as 404 while keeping the original error for logs.
type containedFS struct {
	fsys fs.FS
}

// Open delegates to the root filesystem. Regular files are returned
// untouched so http.FileServerFS still sees their io.Seeker implementation
// (needed for ranges); directories are wrapped so listings omit dotfiles.
func (c containedFS) Open(name string) (fs.File, error) {
	f, err := c.fsys.Open(name)
	if err != nil {
		return nil, notFound(err)
	}
	if d, ok := f.(fs.ReadDirFile); ok {
		if info, err := d.Stat(); err == nil && info.IsDir() {
			return hiddenDotfilesDir{d}, nil
		}
	}
	return f, nil
}

// hiddenDotfilesDir omits dot-prefixed entries from directory listings.
// blockDotfiles already refuses to serve them, but the generated index
// still advertised names such as .env and .git to every client.
type hiddenDotfilesDir struct {
	fs.ReadDirFile
}

func (d hiddenDotfilesDir) ReadDir(n int) ([]fs.DirEntry, error) {
	for {
		entries, err := d.ReadDirFile.ReadDir(n)
		kept := entries[:0]
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), ".") {
				kept = append(kept, e)
			}
		}
		// With n > 0 an all-dotfile batch would return no entries and no
		// error, which callers read as "call again" at best; keep reading
		// until something survives the filter or the directory is done.
		if n <= 0 || len(kept) > 0 || err != nil {
			return kept, err
		}
	}
}

// notFound rewrites an error the root refused into one that satisfies
// errors.Is(err, fs.ErrNotExist), leaving genuine not-exist and permission
// errors alone so net/http keeps mapping them as it already does.
func notFound(err error) error {
	if err == nil || errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		return err
	}
	return fmt.Errorf("%w: %w", fs.ErrNotExist, err)
}

// openServedRoot opens dir as a traversal-resistant root. It fails when dir
// does not exist or is not a directory, so a bad -d is reported at startup
// rather than as 404s at request time.
func openServedRoot(dir string) (*os.Root, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("error opening served directory: %w", err)
	}
	return root, nil
}
