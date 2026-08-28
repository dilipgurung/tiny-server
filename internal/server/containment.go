package server

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
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

// Open delegates to the root filesystem, returning the underlying file
// untouched so http.FileServerFS still sees its fs.ReadDirFile and
// io.Seeker implementations (needed for directory listings and ranges).
func (c containedFS) Open(name string) (fs.File, error) {
	f, err := c.fsys.Open(name)
	if err != nil {
		return nil, notFound(err)
	}
	return f, nil
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
