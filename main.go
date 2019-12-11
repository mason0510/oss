package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"oss/utils"
	"oss/utils/ratelimit"
	"strings"
	"time"
)

const (
	BaseFilePath string = "/home/oss/"
	DefaultRate  int64  = 256 * 1024
)

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
			filePath = fmt.Sprintf(BaseFilePath+"%s/%s", project, timeFmt)
			savePath = fmt.Sprintf(filePath+"%s", uploadHeader.Filename)
		} else {
			filePath = fmt.Sprintf(BaseFilePath+"%s/%s/%s", project, module, timeFmt)
			savePath = fmt.Sprintf(filePath+"%s", uploadHeader.Filename)
		}
		if !utils.FileExists(filePath) {
			_ = os.MkdirAll(filePath, 0775)
		}
		// 3.保存文件到本地目录
		if saveFile, err = os.Create(savePath); err != nil {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]保存文件失败1"}, writer)
			return
		}
		defer saveFile.Close()
		if _, err = io.Copy(saveFile, uploadFile); err != nil {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]保存文件失败2"}, writer)
			return
		}
		// 4.保存成功返回文件路径
		ReturnJson(ResMsg{Code: 200, Data: strings.Replace(savePath, BaseFilePath, "", 1), Msg: "[success]"}, writer)
		return
	}
}

func DownloadHandler(writer http.ResponseWriter, request *http.Request) {
	var (
		err  error
		file *os.File
	)
	// 设置输出流类型
	writer.Header().Set("Content-Type", "application/octet-stream")
	writer.Header().Set("Content-Disposition", "attachment")
	filePath := BaseFilePath + strings.Replace(request.RequestURI, "/oss/api/v1/download/", "", 1)
	if file, err = os.Open(filePath); err != nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]读取文件出现异常"}, writer)
		return
	}
	if file == nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]文件不存在"}, writer)
		return
	}
	// 下载文件，默认限速255KB/s
	bucket := ratelimit.New(DefaultRate)
	if _, err = io.Copy(ratelimit.Writer(writer, bucket), file); err != nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]下载文件出现异常"}, writer)
		return
	}
}

func main() {
	// 初始化数据库
	// 初始化HTTP连接
	http.HandleFunc("/oss/api/v1/upload/", UploadHandler)
	http.HandleFunc("/oss/api/v1/download/", DownloadHandler)
	_ = http.ListenAndServe(":8000", nil)
}
