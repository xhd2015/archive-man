package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanoberholster/imagemeta"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
	"github.com/xhd2015/xgo/support/cmd"
)

func handleExif(args []string) error {
	if len(args) == 0 || args[0] == "" {
		return fmt.Errorf("requires cmd")
	}
	cmd := args[0]
	args = args[1:]
	if cmd == "print-exif" {
		var useImagemeta bool
		var useExifTool bool
		var remainArgs []string
		n := len(args)
		for i := 0; i < n; i++ {
			arg := args[i]
			if arg == "--use-imagemeta" {
				useImagemeta = true
				continue
			}
			if arg == "--use-exiftool" {
				useExifTool = true
				continue
			}
			if !strings.HasPrefix(arg, "-") {
				remainArgs = append(remainArgs, arg)
				continue
			}
			return fmt.Errorf("unrecognized flag: %s", arg)
		}
		if len(remainArgs) == 0 {
			return fmt.Errorf("requires file")
		}
		return printExif(remainArgs[0], &printOpts{
			useImagemeta: useImagemeta,
			useExifTool:  useExifTool,
		})
	}
	if cmd == "print-exif-create-time" {
		var remainArgs []string
		var excludePrefix []string
		n := len(args)
		for i := 0; i < n; i++ {
			arg := args[i]
			if arg == "--exclude-prefix" {
				if i >= n {
					return fmt.Errorf("%s requires arg", arg)
				}
				excludePrefix = append(excludePrefix, args[i+1])
				i++
				continue
			}
			if !strings.HasPrefix(arg, "-") {
				remainArgs = append(remainArgs, arg)
				continue
			}
			return fmt.Errorf("unrecognized flag: %s", arg)
		}
		if len(remainArgs) == 0 {
			return fmt.Errorf("requires file or directory")
		}
		return printExifCreateTime(remainArgs, &printCreateOptions{
			excludePrefix: excludePrefix,
		})
	}
	return fmt.Errorf("unrecognized cmd: %s", cmd)
}

type printOpts struct {
	useImagemeta bool
	useExifTool  bool
}

func printExif(file string, opts *printOpts) error {
	if opts == nil {
		opts = &printOpts{}
	}
	if opts.useExifTool {
		// if exiftool not found, can be installed via brew install exiftool
		return cmd.Run("exiftool", "-a", "-u", "-json", file)
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	if opts.useImagemeta {
		e, err := imagemeta.Decode(f)
		if err != nil {
			return err
		}
		fmt.Println(e)
		return nil
	}
	metadata, err := exif.Decode(f)
	if err != nil {
		log.Fatal(err)
	}

	var p Printer
	return metadata.Walk(p)
}

type Printer struct{}

func (p Printer) Walk(name exif.FieldName, tag *tiff.Tag) error {
	fmt.Printf("%40s: %s\n", name, tag)
	return nil
}

type printCreateOptions struct {
	excludePrefix []string
}

func printExifCreateTime(files []string, opts *printCreateOptions) error {
	if opts == nil {
		opts = &printCreateOptions{}
	}
	// /Users/xhd2015/Volumns/exFAT/iCloud_Photos/2020/un_A
	var actFiles []string
	for _, file := range files {
		ok, err := isDir(file)
		if err != nil {
			return err
		}
		if !ok {
			actFiles = append(actFiles, file)
			continue
		}
		subFiles, err := listDirFiles(file)
		if err != nil {
			return err
		}
		actFiles = append(actFiles, subFiles...)
	}
	for _, file := range actFiles {
		if len(opts.excludePrefix) > 0 {
			name := filepath.Base(file)
			var matchPrefix bool
			for _, prefix := range opts.excludePrefix {
				if strings.HasPrefix(name, prefix) {
					matchPrefix = true
					break
				}
			}
			if matchPrefix {
				continue
			}
		}
		createTime, err := getExifCreateTime(file)
		if err != nil {
			return err
		}
		if createTime == "" {
			createTime = "unknown"
		}
		fmt.Printf("%s: %s\n", file, createTime)
	}
	return nil
}

func getExifCreateTime(file string) (string, error) {
	data, err := cmd.Output("exiftool", "-a", "-u", "-json", file)
	if err != nil {
		return "", err
	}
	var v interface{}
	err = json.Unmarshal([]byte(data), &v)
	if err != nil {
		return "", fmt.Errorf("parsing exiftool output: %w", err)
	}
	// FileModifyDate: no date
	val, ok := tryReadFields(v, []string{"CreateDate" /*JPEG*/, "DateCreated" /*PNG*/, "CreationDate" /*MOV*/})
	if !ok {
		return "", nil
	}
	return fmt.Sprint(val), nil
}

func tryReadFields(v interface{}, fields []string) (interface{}, bool) {
	switch v := v.(type) {
	case map[string]interface{}:
		for _, f := range fields {
			val, ok := v[f]
			if ok {
				return val, true
			}
		}
	case []interface{}:
		for _, x := range v {
			f, ok := tryReadFields(x, fields)
			if ok {
				return f, true
			}
		}
	}
	return nil, false
}

func isDir(s string) (bool, error) {
	stat, err := os.Stat(s)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		return false, nil
	}
	return stat.IsDir(), nil
}

func listDirFiles(s string) ([]string, error) {
	entries, err := os.ReadDir(s)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		files = append(files, filepath.Join(s, name))
	}
	return files, nil
}
