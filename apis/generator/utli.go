package main

import (
	"github.com/pkg/errors"
	"os"
	"strings"
	"unicode"
)

func capitalize(str string) string {
	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func createGolangName(s string) string {
	namearry := strings.Split(s, "_")
	for i, n := range namearry {
		namearry[i] = capitalize(n)
	}
	return strings.Join(namearry, "")
}

func createJSONStructTagName(s string) string {
	namearry := strings.Split(s, "_")
	for i, n := range namearry {
		if i != 0 {
			namearry[i] = capitalize(n)
		}
	}
	return strings.Join(namearry, "")
}

// Checks if a file/folder exists on the given path
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func createFolderIfNotExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.Mkdir(dir, os.ModePerm); err != nil {
			return errors.Wrap(err, "cannot create folder")
		}
	}

	return nil
}
