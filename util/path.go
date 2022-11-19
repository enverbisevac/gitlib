// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"errors"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
)

// EnsureAbsolutePath ensure that a path is absolute, making it
// relative to absoluteBase if necessary
func EnsureAbsolutePath(path, absoluteBase string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(absoluteBase, path)
}

const notRegularFileMode os.FileMode = os.ModeSymlink | os.ModeNamedPipe | os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeIrregular

// GetDirectorySize returns the disk consumption for a given path
func GetDirectorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if info != nil && (info.Mode()&notRegularFileMode) == 0 {
			size += info.Size()
		}
		return err
	})
	return size, err
}

// IsDir returns true if given path is a directory,
// or returns false when it's a file or does not exist.
func IsDir(dir string) (bool, error) {
	f, err := os.Stat(dir)
	if err == nil {
		return f.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsFile returns true if given path is a file,
// or returns false when it's a directory or does not exist.
func IsFile(filePath string) (bool, error) {
	f, err := os.Stat(filePath)
	if err == nil {
		return !f.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// IsExist checks whether a file or directory exists.
// It returns false when the file or directory does not exist.
func IsExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil || os.IsExist(err) {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func statDir(dirPath, recPath string, includeDir, isDirOnly, followSymlinks bool) ([]string, error) {
	dir, err := os.Open(dirPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	fis, err := dir.Readdir(0)
	if err != nil {
		return nil, err
	}

	statList := make([]string, 0)
	for _, fi := range fis {
		if CommonSkip(fi.Name()) {
			continue
		}

		relPath := path.Join(recPath, fi.Name())
		curPath := path.Join(dirPath, fi.Name())
		if fi.IsDir() {
			if includeDir {
				statList = append(statList, relPath+"/")
			}
			s, err := statDir(curPath, relPath, includeDir, isDirOnly, followSymlinks)
			if err != nil {
				return nil, err
			}
			statList = append(statList, s...)
		} else if !isDirOnly {
			statList = append(statList, relPath)
		} else if followSymlinks && fi.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(curPath)
			if err != nil {
				return nil, err
			}

			isDir, err := IsDir(link)
			if err != nil {
				return nil, err
			}
			if isDir {
				if includeDir {
					statList = append(statList, relPath+"/")
				}
				s, err := statDir(curPath, relPath, includeDir, isDirOnly, followSymlinks)
				if err != nil {
					return nil, err
				}
				statList = append(statList, s...)
			}
		}
	}
	return statList, nil
}

// StatDir gathers information of given directory by depth-first.
// It returns slice of file list and includes subdirectories if enabled;
// it returns error and nil slice when error occurs in underlying functions,
// or given path is not a directory or does not exist.
//
// Slice does not include given path itself.
// If subdirectories is enabled, they will have suffix '/'.
func StatDir(rootPath string, includeDir ...bool) ([]string, error) {
	if isDir, err := IsDir(rootPath); err != nil {
		return nil, err
	} else if !isDir {
		return nil, errors.New("not a directory or does not exist: " + rootPath)
	}

	isIncludeDir := false
	if len(includeDir) != 0 {
		isIncludeDir = includeDir[0]
	}
	return statDir(rootPath, "", isIncludeDir, false, false)
}

func isOSWindows() bool {
	return runtime.GOOS == "windows"
}

// FileURLToPath extracts the path information from a file://... url.
func FileURLToPath(u *url.URL) (string, error) {
	if u.Scheme != "file" {
		return "", errors.New("URL scheme is not 'file': " + u.String())
	}

	path := u.Path

	if !isOSWindows() {
		return path, nil
	}

	// If it looks like there's a Windows drive letter at the beginning, strip off the leading slash.
	re := regexp.MustCompile("/[A-Za-z]:/")
	if re.MatchString(path) {
		return path[1:], nil
	}
	return path, nil
}

// HomeDir returns path of '~'(in Linux) on Windows,
// it returns error when the variable does not exist.
func HomeDir() (home string, err error) {
	// TODO: some users run Gitea with mismatched uid  and "HOME=xxx" (they set HOME=xxx by environment manually)
	// TODO: when running gitea as a sub command inside git, the HOME directory is not the user's home directory
	// so at the moment we can not use `user.Current().HomeDir`
	if isOSWindows() {
		home = os.Getenv("USERPROFILE")
		if home == "" {
			home = os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		}
	} else {
		home = os.Getenv("HOME")
	}

	if home == "" {
		return "", errors.New("cannot get home directory")
	}

	return home, nil
}

// CommonSkip will check a provided name to see if it represents file or directory that should not be watched
func CommonSkip(name string) bool {
	if name == "" {
		return true
	}

	switch name[0] {
	case '.':
		return true
	case 't', 'T':
		return name[1:] == "humbs.db"
	case 'd', 'D':
		return name[1:] == "esktop.ini"
	}

	return false
}
