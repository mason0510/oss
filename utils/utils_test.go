package utils

import (
	"fmt"
	"testing"
)

func TestListFile(t *testing.T) {
	fileList, err := ListFile("/home/oss/storage/")
	if err == nil {
		fmt.Println(fileList)
	}
}
