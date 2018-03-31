package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

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

	var ignored, errors, successes uint64
	for _, f := range files {
		go func(f os.FileInfo) {
			fName := strings.TrimSpace(f.Name())
			if !strings.ContainsAny(fName, "[ & ]") {
				atomic.AddUint64(&errors, 1)
				log.Printf("invalid dir named %s! skipping...", fName)
				return
			}
			folderNames := flags.Regex.FindStringSubmatch(fName)
			if folderNames == nil {
				atomic.AddUint64(&errors, 1)
				log.Printf("Could not parse folder %s!", fName)
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
					atomic.AddUint64(&ignored, 1)
					return
				}
			}

			for _, v := range flags.IgnoredChildren {
				if strings.Contains(childName, v) && v != "" {
					atomic.AddUint64(&ignored, 1)
					return
				}
			}

			src, err := filepath.Abs(filepath.Join(inDirName, f.Name()))
			if err != nil {
				atomic.AddUint64(&errors, 1)
				log.Printf("Could not parse folder %s! %s", fName, err)
				return
			}
			destParent, err := filepath.Abs(filepath.Join(outDirName, parentName))
			if err != nil {
				atomic.AddUint64(&errors, 1)
				log.Printf("Could not parse folder %s! %s", parentName, err)
				return
			}
			dest, err := filepath.Abs(filepath.Join(outDirName, parentName, childName))
			if err != nil {
				atomic.AddUint64(&errors, 1)
				log.Printf("Could not parse folder %s! %s", childName, err)
				return
			}

			if err = os.MkdirAll(destParent, f.Mode()); err != nil {
				atomic.AddUint64(&errors, 1)
				log.Printf("Could not create destination folder %s! %s", destParent, err)
				return
			}

			destInfo, err := os.Lstat(dest)
			if err != nil && !os.IsNotExist(err) {
				atomic.AddUint64(&errors, 1)
				log.Printf("Destination not created! %s", err)
				return
			}
			//If a symlink is already there, just delete it
			if destInfo != nil && destInfo.Mode()&os.ModeSymlink != 0 {
				if err := os.Remove(dest); err != nil {
					atomic.AddUint64(&errors, 1)
					log.Printf("Failed to create symlink! %s", err)
					return
				}
			}

			if err := os.Symlink(src, dest); err != nil {
				atomic.AddUint64(&errors, 1)
				log.Printf("\nError symlinking dir %s to %s!\nError:%s", src, dest, err)
				return
			}
			atomic.AddUint64(&successes, 1)
		}(f)
	}
	//wait for goroutines to finish
	for atomic.LoadUint64(&ignored)+atomic.LoadUint64(&errors)+atomic.LoadUint64(&successes) < uint64(len(files)) {
		time.Sleep(10 * time.Millisecond)
	}
	log.Printf("Finished linking %d folders! Ignored: %d Errors: %d ", successes, ignored, errors)
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
