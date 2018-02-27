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

type Flags struct {
	RegexString           string
	Regex                 *regexp.Regexp
	IgnoredParents        []string
	IgnoredParentsString  string
	IgnoredChildren       []string
	IgnoredChildrenString string
}

func consolidateFolders(flags Flags, inDirName, outDirName string) (err error) {
	inDirName = filepath.Clean(inDirName)
	outDirName = filepath.Clean(outDirName)
	files, err := ioutil.ReadDir(inDirName)
	if err != nil {
		return
	}

	errors := make(chan error, len(files))
	ignored := 0
	for _, f := range files {
		go func(f os.FileInfo) {
			fName := strings.TrimSpace(f.Name())
			if !strings.ContainsAny(fName, "[ & ]") {
				errors <- fmt.Errorf("invalid dir named %s! skipping...", fName)
				return
			}
			folderNames := flags.Regex.FindStringSubmatch(fName)
			if folderNames == nil {
				errors <- fmt.Errorf("Could not parse folder %s!", fName)
				return
			}
			var parentName, childName string
			for i, v := range flags.Regex.SubexpNames() {
				if v == "parent" {
					parentName = strings.ToLower(folderNames[i])
				} else if v == "child" {
					childName = strings.TrimSpace(folderNames[i])
				}
			}

			for _, v := range flags.IgnoredParents {
				if strings.Contains(parentName, v) && v != "" {
					ignored++
					errors <- nil
					return
				}
			}

			for _, v := range flags.IgnoredChildren {
				if strings.Contains(childName, v) && v != "" {
					ignored++
					errors <- nil
					return
				}
			}

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
	log.Printf("Linked %d folders with %d errors!", len(files)-ignored-errCount, errCount)
	return
}

func main() {
	var flags Flags
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
			reg := regexp.MustCompile(flags.RegexString)
			if reg.NumSubexp() != 2 {
				log.Fatalln("Regex missing parameter groups!")
			}
			flags.Regex = reg

			flags.IgnoredChildren = strings.Split(flags.IgnoredChildrenString, " ")
			flags.IgnoredParents = strings.Split(flags.IgnoredParentsString, " ")

			if err := consolidateFolders(flags, args[0], args[1]); err != nil {
				log.Fatalf("Error:%s", err)
			}
		},
	}
	rootCLI.PersistentFlags().StringVarP(&flags.RegexString, "regex", "r", `\[(?P<parent>.+?)\](?P<child>.+)`, "Regex for creating tree via 2 named capture groups called parent and child.")
	rootCLI.PersistentFlags().StringVar(&flags.IgnoredParentsString, "ignore-parents", "", "Skips making any symlink parent that contains this string. Space delimited")
	rootCLI.PersistentFlags().StringVar(&flags.IgnoredChildrenString, "ignore-children", "", "Skips making any symlink parent that contains this string. Space delimited")

	if err := rootCLI.Execute(); err != nil {
		log.Fatalf("Failure because %s!", err)
	}
}
