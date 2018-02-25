package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

func consolidateFolders(regex *regexp.Regexp, inDirName, outDirName string) (err error) {
	inDirName = filepath.Clean(inDirName)
	outDirName = filepath.Clean(outDirName)
	files, err := ioutil.ReadDir(inDirName)
	if err != nil {
		return
	}

	errors := make(chan error, len(files))

	for _, f := range files {
		go func(f os.FileInfo) {
			fName := strings.TrimSpace(f.Name())
			if !strings.ContainsAny(fName, "[ & ]") {
				errors <- fmt.Errorf("invalid dir named %s! skipping...", fName)
				return
			}
			folderNames := regex.FindStringSubmatch(fName)
			if folderNames == nil {
				errors <- fmt.Errorf("Could not parse folder %s!", fName)
				return
			}
			parentName := strings.ToLower(folderNames[1])
			childName := strings.TrimSpace(folderNames[2])

			src, _ := filepath.Abs(filepath.Join(inDirName, f.Name()))
			destParent, _ := filepath.Abs(filepath.Join(outDirName, parentName))
			dest, _ := filepath.Abs(filepath.Join(outDirName, parentName, childName))

			if err = os.MkdirAll(destParent, f.Mode()); err != nil {
				return
			}

			destInfo, err := os.Lstat(dest)
			if err != nil && !os.IsNotExist(err) {
				errors <- err
				return
			}
			//If a symlink is already there, just delete it
			if destInfo != nil && destInfo.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(dest); err != nil {
					errors <- err
					return
				}
			}

			if err := os.Symlink(src, dest); err != nil {
				errors <- fmt.Errorf("\nError symlinking dir %s to %s!\nError:%s", src, dest, err)
				return
			}
			// log.Printf("Successfully linked to %s!", dest)
			errors <- nil
		}(f)
	}
	errCount := 0
	for i := 0; i < len(files); i++ {
		e := <-errors
		if e != nil {
			errCount++
			log.Print(e)
		}
	}
	log.Printf("Linked %d folders with %d errors!", len(files), errCount)
	return
}

func main() {
	var regex string
	rootCLI := &cobra.Command{
		Use:   "dir-tree inputDir outputDir",
		Short: "Restructure a folders subfolders via a regex pattern in their names.",
		Long: `
		If you had an inputDir that looks like so... 
			Music -> [Big Shaq] Mans not Hot
                              -> [Big Shaq] Fire in the booth
		The outputDir could look like...
			MusicByAuthor -> big shaq -> Mans not Hot
                                                  -> Fire in the booth
		`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			reg := regexp.MustCompile(regex)
			if err := consolidateFolders(reg, args[0], args[1]); err != nil {
				log.Fatalf("Error:%s", err)
			}
		},
	}
	rootCLI.PersistentFlags().StringVarP(&regex, "regex", "r", "\\[(.+?)\\](.+)", "Regex for splitting subdir according to 2 capture groups. The first will be used to create the directory under outputDir, second will be used for the subdirectory under that.")

	if err := rootCLI.Execute(); err != nil {
		log.Fatalf("Failure because %s!", err)
	}
}
