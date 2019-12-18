package utils

import (
	"fmt"
	"path"
	"testing"
)

func TestListFile(t *testing.T) {
	fileList, err := ListFile("/home/oss/storage/")
	if err == nil {
		fmt.Println(fileList)
	}
}

func TestGetUUID(t *testing.T) {
	fmt.Println(GetUUID() + path.Ext("投标文件模板.2019.12.18.doc"))
	fmt.Println(MD5(GetUUID()) + path.Ext("投标文件模板.2019.12.18.doc"))
}
