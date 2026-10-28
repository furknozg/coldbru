// Command gen writes internal/proposal/prompts/prompts.lock.json (D-059). Run it from the
// repository root with `make generate-prompts`. It refuses to record a changed prompt hash under an
// unchanged version: raise the version in the package's manifest.yaml first.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"

	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

func main() {
	out := flag.String("out", "internal/proposal/prompts/"+prompts.LockFile, "lock file to write")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "prompts gen:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	var committed prompts.Lock
	switch b, err := os.ReadFile(path); {
	case err == nil:
		if committed, err = prompts.ParseLock(b); err != nil {
			return err
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	next, err := committed.Regenerate(prompts.CurrentLock())
	if err != nil {
		return err
	}
	return os.WriteFile(path, next.Marshal(), 0o644)
}
