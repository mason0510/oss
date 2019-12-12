package utils

import (
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

func FileExists(fileName string) bool {
	_, err := os.Stat(fileName)
	return err == nil
}

func ListFile(baseFolder string) (fileList []string, err error) {
	err = filepath.Walk(baseFolder, func(path string, info os.FileInfo, err error) error {
		if info != nil && info.IsDir() == false {
			fileList = append(fileList, strings.Replace(path, "\\", "/", -1))
		}
		return nil
	})
	return fileList, err
}

func Bytes2Str(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
