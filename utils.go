package main

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"flag"
	"io"
	"os"
	"path"
	"regexp"

	"github.com/denisbrodbeck/machineid"
	"github.com/google/uuid"
)

func FileExists(filePath string) (bool, error) {
	if _, err := os.Stat(filePath); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		return false, err
	}
}

func FileReadBytes(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return fileBytes, nil
}

func FileReadString(filePath string) (string, error) {
	fileBytes, err := FileReadBytes(filePath)
	if err != nil {
		return "", err
	}
	return string(fileBytes), err
}

func FilesList(dirPath string, include string, ignore string) ([]string, error) {
	var files []string = make([]string, 0)

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return files, err
	}

	var includeRegexp *regexp.Regexp
	if include == "" {
		includeRegexp = nil
	} else {
		includeRegexp, err = regexp.Compile(include)
		if err != nil {
			return files, err
		}
	}

	var ignoreRegexp *regexp.Regexp
	if ignore == "" {
		ignoreRegexp = nil
	} else {
		ignoreRegexp, err = regexp.Compile(ignore)
		if err != nil {
			return files, err
		}
	}

	for _, subdirEntry := range entries {
		err := filesListInDirEntry(dirPath, subdirEntry, includeRegexp, ignoreRegexp, &files)
		if err != nil {
			return files, err
		}
	}

	return files, nil
}

func filesListInDirEntry(parentPath string, dirEntry os.DirEntry, includeRegexp *regexp.Regexp, ignoreRegexp *regexp.Regexp, acc *[]string) error {
	fullPath := path.Join(parentPath, dirEntry.Name())

	if ignoreRegexp != nil && ignoreRegexp.MatchString(fullPath) {
		return nil
	}

	if !dirEntry.IsDir() && (includeRegexp == nil || includeRegexp.MatchString(fullPath)) {
		*acc = append(*acc, fullPath)
		return nil
	}

	if dirEntry.IsDir() {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return err
		}
		for _, subdirEntry := range entries {
			filesListInDirEntry(fullPath, subdirEntry, includeRegexp, ignoreRegexp, acc)
		}
	}

	return nil
}

func FlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

// 256 bit system id (32 bytes)
func SystemId() ([]byte, error) {
	systemId, err := machineid.ID()
	if err != nil {
		return nil, err
	}
	systemUUID, err := uuid.Parse(systemId)
	if err != nil {
		return nil, err
	}
	systemUUIDBytes, err := systemUUID.MarshalBinary()
	if err != nil {
		return nil, err
	}
	systemIdHash := sha256.New()
	systemIdHash.Write(systemUUIDBytes)
	systemIdHashBytes := systemIdHash.Sum(nil)
	return systemIdHashBytes, nil
}

// base64 encoded 256 bit system id (32 bytes)
func SystemIdString() (string, error) {
	systemIdBytes, err := SystemId()
	if err != nil {
		return "", err
	}
	systemId := base64.StdEncoding.EncodeToString(systemIdBytes)
	return systemId, nil
}
