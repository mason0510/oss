package compress

import (
	"fmt"
	"testing"
)

func TestGenShortUrl(t *testing.T) {
	url := "sdk/20191210/18/中国"
	cb := func(url, keyword string) bool {
		return true
	}
	sUrl := GenShortUrl(CharsetRandomAlphanumeric, url, cb)
	fmt.Println(sUrl)
}
