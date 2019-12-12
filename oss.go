package main

import (
	"encoding/json"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"oss/utils"
	"oss/utils/compress"
	"oss/utils/ratelimit"
	"strconv"
	"strings"
	"time"
)

const (
	FileIndexPath   string = "/home/oss/index/"
	BaseStoragePath string = "/home/oss/storage/"
	DefaultRate     int64  = 256 * 1024
)

var (
	dbConn *leveldb.DB
)

const UploadHtml string = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<title>测试上传文件</title>
</head>
<body>
<form action="http://localhost:8000/oss/api/v1/upload" method="post" enctype="multipart/form-data">
	<label>项目名称（必填）:
		<input type="text" name="project"/>
	</label>
	<label>模块名称（非必填）:
		<input type="text" name="module"/>
	</label>
	<input type="file" name="file"/>
	<input type="submit" value="上传"/>
</form>
</body>
</html>`

type ResMsg struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

func ReturnJson(resMsg ResMsg, write http.ResponseWriter) {
	// 返回JSON数据
	resMsgJson, _ := json.Marshal(resMsg)
	write.Header().Set("Content-Type", "application/json")
	_, _ = write.Write(resMsgJson)
}

// 上传文件页面
func UploadHtmlHandler(writer http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(UploadHtml))
	}
}

// 上传文件
func UploadHandler(writer http.ResponseWriter, request *http.Request) {
	// 设置允许跨域访问
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Depth, User-Agent, X-File-Size, X-Requested-With, X-Requested-By, If-Modified-Since, X-File-Name, X-File-Type, Cache-Control, Origin")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	writer.Header().Set("Access-Control-Expose-Headers", "Authorization")
	// 可能需要校验用户信息
	var (
		err          error
		project      string
		module       = ""
		uploadFile   multipart.File
		uploadHeader *multipart.FileHeader
		filePath     string
		savePath     string
		saveFile     *os.File
	)
	if request.Method == http.MethodPost {
		// 0.获取
		project = request.FormValue("project")
		if len(project) == 0 {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]project不能为空"}, writer)
			return
		}
		module = request.FormValue("module")
		// 1.获取文件信息
		if uploadFile, uploadHeader, err = request.FormFile("file"); err != nil {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]获取文件失败"}, writer)
			return
		}
		defer uploadFile.Close()
		// 2.构造文件存储路径，可以很方便的按照天进行数据同步
		timeFmt := time.Unix(time.Now().Unix(), 0).Format("20060102/15/")
		if len(module) == 0 {
			filePath = fmt.Sprintf(BaseStoragePath+"%s/%s", project, timeFmt)
			savePath = fmt.Sprintf(filePath+"%s", uploadHeader.Filename)
		} else {
			filePath = fmt.Sprintf(BaseStoragePath+"%s/%s/%s", project, module, timeFmt)
			savePath = fmt.Sprintf(filePath+"%s", uploadHeader.Filename)
		}
		if _, err = os.Stat(savePath); err == nil { // 判断文件是否存在
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]文件名重复，请重新上传"}, writer)
			return
		}
		if !utils.FileExists(filePath) { // 是否需要创建文件夹
			_ = os.MkdirAll(filePath, 0775)
		}
		// 3.保存文件到本地目录
		if saveFile, err = os.Create(savePath); err != nil {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]保存文件失败1"}, writer)
			return
		}
		defer saveFile.Close()
		buffer := make([]byte, 1024)
		if _, err = io.CopyBuffer(saveFile, uploadFile, buffer); err != nil {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]保存文件失败2"}, writer)
			return
		}
		// 4.保存文件路径和索引到数据库
		sUrl := compress.GenShortUrl(compress.CharsetRandomAlphanumeric, savePath, func(url, keyword string) bool {
			data, _ := dbConn.Get([]byte(keyword), nil)
			if data == nil {
				return true
			}
			return false
		})
		err = dbConn.Put([]byte(sUrl), []byte(savePath), nil)
		// 5.返回文件唯一索引
		ReturnJson(ResMsg{Code: 200, Data: sUrl, Msg: "[success]"}, writer)
		return
	}
}

// 下载文件
func DownloadHandler(writer http.ResponseWriter, request *http.Request) {
	var (
		err      error
		filePath []byte
		file     *os.File
		fileInfo os.FileInfo
		fileName string
	)
	filePathKey := strings.Replace(request.RequestURI, "/oss/api/v1/download/", "", 1)
	if filePath, err = dbConn.Get([]byte(filePathKey), nil); err != nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]读取文件路径出现异常"}, writer)
		return
	}
	if file, err = os.Open(utils.Bytes2Str(filePath)); err != nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]读取文件出现异常"}, writer)
		return
	}
	defer file.Close()
	if file == nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]文件不存在"}, writer)
		return
	}
	if fileInfo, err = file.Stat(); err != nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]读取文件失败"}, writer)
		return
	}
	// 设置输出流类型
	fileName = fileInfo.Name()
	fileName = url.QueryEscape(fileName)
	writer.Header().Set("Content-Type", "application/octet-stream")
	writer.Header().Set("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	writer.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	// 下载文件，默认限速255KB/s
	bucket := ratelimit.New(DefaultRate)
	buffer := make([]byte, 1024)
	if _, err = io.CopyBuffer(ratelimit.Writer(writer, bucket), file, buffer); err != nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]下载文件出现异常"}, writer)
		return
	}
}

// 同步文件索引
func SyncFileIndex() {
	fileList, err := utils.ListFile(BaseStoragePath)
	if err == nil {
		for i := 0; i < len(fileList); i++ {
			savePath := fileList[i]
			sUrl := compress.GenShortUrl(compress.CharsetRandomAlphanumeric, savePath, func(url, keyword string) bool {
				data, _ := dbConn.Get([]byte(keyword), nil)
				if data == nil {
					return true
				}
				return false
			})
			err = dbConn.Put([]byte(sUrl), []byte(savePath), nil)
		}
	}
}

func main() {
	// 读取配置文件
	// 初始化数据库
	dbConn, _ = leveldb.OpenFile(FileIndexPath, nil)
	defer dbConn.Close()
	// 初始化HTTP连接
	http.HandleFunc("/oss/upload.html", UploadHtmlHandler)
	http.HandleFunc("/oss/api/v1/upload", UploadHandler)
	http.HandleFunc("/oss/api/v1/download/", DownloadHandler)
	_ = http.ListenAndServe(":8000", nil)
}
