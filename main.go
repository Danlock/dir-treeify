package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func copyFile(from, to string) (err error) {
	in, err := os.Open(from)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(to)
	if err != nil {
		return
	}
	defer func() {
		if e := out.Close(); e != nil {
			err = e
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return
	}

	err = out.Sync()
	if err != nil {
		return
	}

	si, err := os.Stat(from)
	if err != nil {
		return
	}
	err = os.Chmod(to, si.Mode())
	if err != nil {
		return
	}

	return
}

func copyDir(from, to string) (err error) {
	src := filepath.Clean(from)
	dst := filepath.Clean(to)

	si, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !si.IsDir() {
		return fmt.Errorf("source is not a directory")
	}

	_, err = os.Stat(dst)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	if err == nil {
		return fmt.Errorf("destination already exists")
	}

	err = os.MkdirAll(dst, si.Mode())
	if err != nil {
		return
	}

	entries, err := ioutil.ReadDir(src)
	if err != nil {
		return
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = copyDir(srcPath, dstPath)
			if err != nil {
				return
			}
		} else {
			// Skip symlinks.
			if entry.Mode()&os.ModeSymlink != 0 {
				continue
			}

			err = copyFile(srcPath, dstPath)
			if err != nil {
				return
			}
		}
	}

	return
}

func consolidateFolders(inDirName, outDirName string) (err error) {
	files, err := ioutil.ReadDir(inDirName)
	if err != nil {
		log.Fatalf("Failed to read dir!\n%s", err)
	}
	for _, f := range files {
		fName := strings.TrimSpace(f.Name())
		if !strings.ContainsAny(fName, "[ & ]") {
			log.Printf("invalid dir named %s! skipping...", fName)
			continue
		}

		locOfStartingBrace := strings.Index(fName, "[")

		locOfEndingBrace := strings.LastIndex(fName, "]")

		dirName := fName[locOfStartingBrace+1 : locOfEndingBrace-1]
		if _, err := os.Stat(inDirName + dirName); os.IsNotExist(err) {
			if err := os.MkdirAll(inDirName+dirName, os.ModeDir); err != nil {
				log.Fatalf("Error creating dir %s!", dirName)
			}
			log.Printf("made %s dir!", dirName)
		}

	}
	return
}

func main() {
	rootCLI := &cobra.Command{
		Use:   "dir-sorter",
		Short: "Reorganizing directories according to a pattern",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return consolidateFolders(args[0], args[1])
		},
	}

	if err := rootCLI.Execute(); err != nil {
		log.Fatalf("Failure because %s!", err)
	}
}
