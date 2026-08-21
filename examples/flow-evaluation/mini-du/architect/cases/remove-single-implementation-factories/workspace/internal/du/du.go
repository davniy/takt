// Package du implements mini-du, a small disk-usage tool with a fixed
// public contract:
//
//	mini-du [-s] [-k|-H] [--] [PATH...]
//
// Sizes are disk usage in 1024-byte (kibibyte) units computed from the
// allocated blocks reported by the filesystem, matching du -k. -H prints
// the allocated size humanized with binary units.
package du

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const helpText = `Usage: mini-du [-s] [-k|-H] [--] [PATH...]
  -s          display only a total for each path
  -k          display sizes in 1024-byte units
  -H          display humanized binary units (KiB, MiB, GiB)
  -h, --help  display this help
`

// errSilent signals a non-zero exit code after diagnostics were already
// written to stderr by Run.
var errSilent = errors.New("du failed")

type options struct {
	sumOnly bool
	human   bool
	paths   []string
}

// Run executes mini-du with the given arguments, writing results to stdout
// and diagnostics to stderr. It returns a non-nil error when the process
// must exit non-zero; diagnostics are already written to stderr.
func Run(args []string, stdout, stderr io.Writer) error {
	prog := "mini-du"
	opt := &options{}

	afterDashDash := false
	help := false
	unknown := rune(0)
	for _, arg := range args {
		if afterDashDash {
			opt.paths = append(opt.paths, arg)
			continue
		}
		switch {
		case arg == "--":
			afterDashDash = true
		case arg == "-h" || arg == "--help":
			help = true
		case len(arg) > 1 && arg[0] == '-' && arg[1] != '-':
			for i := 1; i < len(arg); i++ {
				switch arg[i] {
				case 's':
					opt.sumOnly = true
				case 'k':
					// -k is an accepted alias for the default unit.
				case 'H':
					opt.human = true
				default:
					if unknown == 0 {
						unknown = rune(arg[i])
					}
				}
			}
		default:
			opt.paths = append(opt.paths, arg)
		}
		if unknown != 0 {
			break
		}
	}
	if unknown != 0 {
		fmt.Fprintf(stderr, "%s: unknown option -%c\n", prog, unknown)
		return errSilent
	}
	if help {
		fmt.Fprint(stdout, helpText)
		return nil
	}
	if len(opt.paths) == 0 {
		opt.paths = []string{"."}
	}

	w := &walker{
		prog:   prog,
		human:  opt.human,
		seen:   make(map[inoKey]bool),
		stdout: stdout,
		stderr: stderr,
	}
	failed := false
	for i, path := range opt.paths {
		if i > 0 {
			w.seen = make(map[inoKey]bool)
		}
		if w.report(path, opt.sumOnly) {
			failed = true
		}
	}
	if failed {
		return errSilent
	}
	return nil
}

type inoKey struct {
	dev uint64
	ino uint64
}

type walker struct {
	prog   string
	human  bool
	seen   map[inoKey]bool
	stdout io.Writer
	stderr io.Writer
}

func (w *walker) diag(path string, err error) {
	fmt.Fprintf(w.stderr, "%s: %s: %v\n", w.prog, path, err)
}

func (w *walker) report(path string, sumOnly bool) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		w.diag(path, err)
		return true
	}

	var total int64
	var bad bool
	switch {
	case fi.IsDir():
		total, bad = w.walkDir(path, !sumOnly)
	case fi.Mode()&os.ModeSymlink != 0:
		total = allocatedSize(fi)
	default:
		total, bad = w.leafSize(path, fi)
	}
	w.println(total, path)
	return bad
}

// walkDir returns the allocated size of the directory tree rooted at path,
// printing child lines post-order when printChildren is true.
func (w *walker) walkDir(path string, printChildren bool) (int64, bool) {
	fi, err := os.Lstat(path)
	if err != nil {
		w.diag(path, err)
		return 0, true
	}
	key, ok := inodeKey(fi)
	if ok && w.seen[key] {
		return 0, false
	}
	if ok {
		w.seen[key] = true
	}

	total := allocatedSize(fi)
	bad := false
	entries, err := os.ReadDir(path)
	if err != nil {
		w.diag(path, err)
		bad = true
	}
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		var sub int64
		var childBad bool
		switch {
		case entry.IsDir():
			sub, childBad = w.walkDir(child, printChildren)
		default:
			cfi, err := os.Lstat(child)
			if err != nil {
				w.diag(child, err)
				childBad = true
				continue
			}
			if cfi.Mode()&os.ModeSymlink != 0 {
				sub = allocatedSize(cfi)
			} else {
				sub, childBad = w.leafSize(child, cfi)
			}
		}
		total += sub
		bad = bad || childBad
		if printChildren && entry.IsDir() {
			w.println(sub, child)
		}
	}
	return total, bad
}

// leafSize returns the allocated size of a non-directory entry, counting
// hard-linked regular files only once.
func (w *walker) leafSize(path string, fi fs.FileInfo) (int64, bool) {
	if !fi.Mode().IsRegular() {
		return 0, false
	}
	key, ok := inodeKey(fi)
	if ok && w.seen[key] {
		return 0, false
	}
	if ok {
		w.seen[key] = true
	}
	return allocatedSize(fi), false
}

func inodeKey(fi fs.FileInfo) (inoKey, bool) {
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return inoKey{}, false
	}
	return inoKey{dev: uint64(sys.Dev), ino: uint64(sys.Ino)}, true
}

// allocatedSize returns the number of bytes allocated to the file on disk,
// based on 512-byte filesystem blocks like du.
func allocatedSize(fi fs.FileInfo) int64 {
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		if fi.Mode().IsRegular() {
			return fi.Size()
		}
		return 0
	}
	return sys.Blocks * 512
}

func (w *walker) println(bytes int64, path string) {
	fmt.Fprintf(w.stdout, "%s\t%s\n", formatSize(bytes, w.human), path)
}

// formatSize renders an allocated byte count as integer kibibytes (matching
// du -k) or, when human is true, as humanized binary units.
func formatSize(bytes int64, human bool) string {
	if human {
		return humanSize(bytes)
	}
	if bytes <= 0 {
		return "0"
	}
	return strconv.FormatInt((bytes+1023)/1024, 10)
}

var humanUnits = []string{"B", "KiB", "MiB", "GiB", "TiB"}

// humanSize renders bytes with binary units: 0B, one decimal place below
// 10, and rounded integers from 10.
func humanSize(bytes int64) string {
	if bytes == 0 {
		return "0B"
	}
	v := float64(bytes)
	i := 0
	for v >= 1024 && i < len(humanUnits)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return strconv.FormatInt(bytes, 10) + "B"
	}
	if v < 10 {
		s := strconv.FormatFloat(v, 'f', 1, 64)
		s = strings.TrimSuffix(s, ".0")
		return s + humanUnits[i]
	}
	return strconv.FormatInt(int64(math.Round(v)), 10) + humanUnits[i]
}
