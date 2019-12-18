package utils

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
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

func GetUUID() string {
	b := make([]byte, 48)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	id := MD5(base64.URLEncoding.EncodeToString(b))
	return fmt.Sprintf("%s-%s-%s-%s-%s", id[0:8], id[8:12], id[12:16], id[16:20], id[20:])
}

func MD5(str string) string {
	md := md5.New()
	md.Write([]byte(str))
	return fmt.Sprintf("%x", md.Sum(nil))
}

func GetFileMd5(file *os.File) string {
	_, _ = file.Seek(0, 0)
	md5h := md5.New()
	_, _ = io.Copy(md5h, file)
	sum := fmt.Sprintf("%x", md5h.Sum(nil))
	return sum
}
