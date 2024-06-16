package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// copy files

// usages:
//
// go run ./ inspect --prefix ._ --count G:\iCloud_Photos
// go run ./ sync --check-duplicate G:\iCloud_Photos D:\iCloud-Photos\
// go run ./ sync G:\iCloud_Photos D:\iCloud-Photos\
func main() {
	args := os.Args[1:]
	err := handle(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

type Op int

const (
	Op_Print     Op = 0
	Op_Delete    Op = 1
	Op_PrintTree Op = 2
)

const op Op = Op_Print

func handle(args []string) error {
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("requires cmd")
	}
	if args[0] == "print-exif" || args[0] == "print-exif-create-time" {
		return handleExif(args)
	}

	cmd := args[0]
	args = args[1:]
	if cmd == "sync" {
		return sync(args)
	}

	if cmd == "inspect" {
		return inspect(args)
	}
	if cmd == "delete-file" || cmd == "delete-files" {
		return deleteFiles(args)
	}

	if true {
		return fmt.Errorf("unrecognized cmd: %s", cmd)
	}

	dir := cmd
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	cnt := 0
	err = filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch op {
		case Op_Delete:
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			lowName := strings.ToLower(name)
			if strings.HasSuffix(lowName, ".jpeg") || strings.HasSuffix(lowName, "jpg") || strings.HasSuffix(lowName, ".png") || strings.HasSuffix(lowName, ".webp") || strings.HasSuffix(lowName, ".mov") || strings.HasSuffix(lowName, ".mp4") || strings.HasSuffix(lowName, ".gif") {
				cnt++
				fmt.Printf("removing %s\n", path)
				os.RemoveAll(path)
				return nil
			}
			fmt.Printf("found extra %s\n", path)
			if true {
				return nil
			}
			if strings.HasPrefix(name, ".") {
				fmt.Printf("found %s\n", path)
				if strings.HasPrefix(name, "._") {
					fmt.Printf("found incomplete %s\n", path)
					os.RemoveAll(path)
				}
			}
		case Op_PrintTree:
			fmt.Printf("found %s\n", path)
		default:
			fmt.Printf("found %s\n", path)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func sync(args []string) error {
	var dryRun bool
	var checkDuplicate bool
	var removeSynced bool
	n := len(args)
	var remainArgs []string
	for i := 0; i < n; i++ {
		arg := args[i]
		if arg == "--dry-run" {
			dryRun = true
			continue
		}
		if arg == "--check-duplicate" {
			checkDuplicate = true
			continue
		}
		if arg == "--remove-synced" {
			removeSynced = true
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			remainArgs = append(remainArgs, arg)
			continue
		}
		return fmt.Errorf("unknown flag: %s", arg)
	}
	// sync src dst
	if len(remainArgs) < 2 {
		return fmt.Errorf("usage: sync SRC DST")
	}
	src := remainArgs[0]
	dst := remainArgs[1]

	var totalDirs int
	var totalFiles int
	err := walkRelative(src, func(path, relPath string, d fs.DirEntry) error {
		dstPath := filepath.Join(dst, relPath)
		if d.IsDir() {
			if checkDuplicate {
				return nil
			}
			if removeSynced {
				return nil
			}
			if dryRun {
				totalDirs++
				fmt.Printf("mkdir %s\n", dstPath)
				return nil
			}
			totalDirs++
			return os.MkdirAll(dstPath, 0755)
		}
		dstExists, err := exists(dstPath)
		if err != nil {
			return err
		}
		if checkDuplicate {
			if dstExists {
				totalFiles++
				fmt.Printf("duplicate %s %s\n", path, dstPath)
			}
			return nil
		}
		if removeSynced {
			if !dstExists {
				return nil
			}
			if dryRun {
				totalFiles++
				fmt.Printf("rm %s\n", path)
				return nil
			}
			return os.Remove(path)
		}
		if dryRun {
			if dstExists {
				return nil
			}
			totalFiles++
			fmt.Printf("cp %s %s\n", path, dstPath)
			return nil
		}
		if dstExists {
			return nil
		}
		totalFiles++
		return copyFile(path, dstPath)
	})
	fmt.Printf("dirs: %d, files: %d\n", totalDirs, totalFiles)
	return err
}

func exists(path string) (bool, error) {
	dstStat, dstErr := os.Stat(path)
	if dstErr != nil {
		if !errors.Is(dstErr, os.ErrNotExist) {
			return false, dstErr
		}
	}
	if dstStat == nil {
		return false, nil
	}
	return true, nil
}

func copyFile(src string, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	srcStat, err := os.Stat(src)
	if err != nil {
		return err
	}
	modTime := srcStat.ModTime()
	return os.Chtimes(dst, time.Time{}, modTime)
}

func inspect(args []string) error {
	return manageDir(manageCommand_Inspect, args)
}

func deleteFiles(args []string) error {
	return manageDir(manageCommand_DeleteFiles, args)
}

type manageCommand int

const (
	manageCommand_Inspect     manageCommand = 0
	manageCommand_DeleteFiles manageCommand = 1
)

func manageDir(cmd manageCommand, args []string) error {
	var limit int
	var count bool

	var dryRun bool

	n := len(args)
	var remainArgs []string
	var prefixes []string
	for i := 0; i < n; i++ {
		arg := args[i]
		if cmd == manageCommand_Inspect {
			if arg == "--count" {
				count = true
				continue
			}
		}
		if cmd == manageCommand_DeleteFiles {
			if arg == "--dry-run" {
				dryRun = true
				continue
			}
		}
		if arg == "--prefix" {
			if i >= n {
				return fmt.Errorf("%s requires arg", arg)
			}
			prefixes = append(prefixes, args[i+1])
			i++
			continue
		}
		if arg == "--limit" {
			if i >= n {
				return fmt.Errorf("%s requires arg", arg)
			}
			num, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil {
				return fmt.Errorf("%s: %w", arg, err)
			}
			limit = int(num)
			i++
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			remainArgs = append(remainArgs, arg)
			continue
		}
		return fmt.Errorf("unknown flag: %s", arg)
	}
	// sync src dst
	if len(remainArgs) < 1 {
		return fmt.Errorf("requires DIR")
	}
	delete := cmd == manageCommand_DeleteFiles
	dir := remainArgs[0]

	var total int
	err := walkRelative(dir, func(path, relPath string, d fs.DirEntry) error {
		name := d.Name()
		if len(prefixes) > 0 {
			var matchPrefix bool
			for _, prefix := range prefixes {
				if strings.HasPrefix(name, prefix) {
					matchPrefix = true
					break
				}
			}
			if !matchPrefix {
				return nil
			}
		}
		if !d.IsDir() {
			total++
			if limit > 0 && total >= limit {
				return filepath.SkipAll
			}
		}
		if delete {
			if d.IsDir() {
				return nil
			}
			if dryRun {
				fmt.Printf("rm %s\n", relPath)
				return nil
			}
			return os.RemoveAll(path)
		} else if !count {
			fmt.Printf("%s\n", relPath)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if delete {
		if dryRun {
			fmt.Printf("will delete: %d\n", total)
		} else {
			fmt.Printf("deleted: %d\n", total)
		}
	} else if count {
		fmt.Printf("%d\n", total)
	}
	return nil
}

func walkRelative(dir string, f func(path string, relPath string, d fs.DirEntry) error) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	return filepath.WalkDir(absDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == absDir {
			return nil
		}
		if !strings.HasPrefix(path, absDir) {
			return fmt.Errorf("unexpected path: %s", path)
		}
		relPath := strings.TrimPrefix(path[len(absDir):], string([]rune{filepath.Separator}))

		return f(path, relPath, d)
	})
}
