package main

import (
	"encoding/json"
	"fmt"
	"github.com/pelletier/go-toml"
	"github.com/syndtr/goleveldb/leveldb"
	"io"
	"mime/multipart"
	. "net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"oss/utils"
	"oss/utils/compress"
	"oss/utils/ratelimit"
	"path"
	"strconv"
	"strings"
	"time"
)

// 不可变参数
const (
	FileIndexPath   string = "/home/oss/index/"
	ConfigPath      string = "/home/oss/config/"
	ConfigFileName  string = "config.toml"
	BaseStoragePath string = "/home/oss/storage/"
)

// 配置文件参数
var (
	dbConn             *leveldb.DB
	config             *toml.Tree
	addr                     = ":8000"
	defaultRate        int64 = 256 << 10
	defaultMaxFileSize int64 = 256 << 20
	rename             bool
)

const UploadHtml string = `
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<title>测试上传文件</title>
</head>
<body>
<form action="/oss/api/v1/upload" method="post" enctype="multipart/form-data">
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

type FileInfo struct {
	Name      string `json:"name"`
	ReName    string `json:"rename"`
	Path      string `json:"path"`
	Md5       string `json:"md5"`
	Size      int64  `json:"size"`
	TimeStamp int64  `json:"timeStamp"`
}

func ReturnJson(resMsg ResMsg, write ResponseWriter) {
	// 返回JSON数据
	resMsgJson, _ := json.Marshal(resMsg)
	write.Header().Set("Content-Type", "application/json")
	_, err := write.Write(resMsgJson)
	fmt.Println(err)
}

// 上传文件页面
func UploadHtmlHandler(writer ResponseWriter, request *Request) {
	if request.Method == MethodGet {
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write([]byte(UploadHtml))
	}
}

// 上传文件
func UploadHandler(writer ResponseWriter, request *Request) {
	var (
		err          error
		project      string
		module       = ""
		fileInfo     FileInfo
		fileInfoJson []byte
		uploadFile   multipart.File
		uploadHeader *multipart.FileHeader
		filePath     string
		fileName     string
		savePath     string
		saveFile     *os.File
	)
	// 设置允许跨域访问
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Depth, User-Agent, X-File-Size, X-Requested-With, X-Requested-By, If-Modified-Since, X-File-Name, X-File-Type, Cache-Control, Origin")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	writer.Header().Set("Access-Control-Expose-Headers", "Authorization")
	// 可能需要校验用户信息
	if request.Method == MethodPost {
		// 0.获取
		project = request.FormValue("project")
		if len(project) == 0 {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]project不能为空"}, writer)
			return
		}
		module = request.FormValue("module")
		// 1.获取文件信息
		if err = request.ParseMultipartForm(32 << 20); err != nil {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]上传文件失败，未知异常"}, writer)
			return
		}
		if request.ContentLength > defaultMaxFileSize {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]上传文件失败，文件过大"}, writer)
			return
		}
		if uploadFile, uploadHeader, err = request.FormFile("file"); err != nil {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]上传文件失败，获取文件异常"}, writer)
			return
		}
		defer uploadFile.Close()
		// 2.构造文件存储路径，可以很方便的按照天进行数据同步
		if rename {
			fileName = utils.MD5(utils.GetUUID()) + path.Ext(uploadHeader.Filename)
		} else {
			fileName = uploadHeader.Filename
		}
		timeFmt := time.Unix(time.Now().Unix(), 0).Format("20060102/15/")
		if len(module) == 0 {
			filePath = fmt.Sprintf(BaseStoragePath+"%s/%s", project, timeFmt)
			savePath = fmt.Sprintf(filePath+"%s", fileName)
		} else {
			filePath = fmt.Sprintf(BaseStoragePath+"%s/%s/%s", project, module, timeFmt)
			savePath = fmt.Sprintf(filePath+"%s", fileName)
		}
		if _, err = os.Stat(savePath); err == nil { // 判断文件是否存在
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]保存文件失败，文件名重复"}, writer)
			return
		}
		if !utils.FileExists(filePath) { // 是否需要创建文件夹
			_ = os.MkdirAll(filePath, 0775)
		}
		// 3.保存文件到本地目录
		if saveFile, err = os.Create(savePath); err != nil {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]保存文件失败，打开文件异常"}, writer)
			return
		}
		defer saveFile.Close()
		buffer := make([]byte, 1024)
		if _, err = io.CopyBuffer(saveFile, uploadFile, buffer); err != nil {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]保存文件失败，写入文件异常"}, writer)
			return
		}
		// 4.构造保存结构
		fileInfo.Name = uploadHeader.Filename
		fileInfo.ReName = fileName
		fileInfo.Path = savePath
		fileInfo.Md5 = utils.GetFileMd5(saveFile)
		fileInfo.Size = request.ContentLength
		fileInfo.TimeStamp = time.Now().UnixNano() / 1e6
		// 5.保存文件路径和索引到数据库
		if fileInfoJson, err = json.Marshal(fileInfo); err != nil {
			ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]保存文件失败，写入文件异常"}, writer)
			return
		}
		sUrl := compress.GenShortUrl(compress.CharsetRandomAlphanumeric, savePath, func(url, keyword string) bool {
			data, _ := dbConn.Get([]byte(keyword), nil)
			if data == nil {
				return true
			}
			return false
		})
		err = dbConn.Put([]byte(sUrl), fileInfoJson, nil)
		// 6.返回文件唯一索引
		ReturnJson(ResMsg{Code: 200, Data: sUrl, Msg: "[success]"}, writer)
		return
	}
}

// 下载文件
func DownloadHandler(writer ResponseWriter, request *Request) {
	var (
		err          error
		fileInfoJson []byte
		file         *os.File
		fileInfo     FileInfo
	)
	filePathKey := strings.Replace(request.RequestURI, "/oss/api/v1/download/", "", 1)
	if fileInfoJson, err = dbConn.Get([]byte(filePathKey), nil); err != nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]读取文件路径出现异常"}, writer)
		return
	}
	if err = json.Unmarshal(fileInfoJson, &fileInfo); err != nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]读取文件信息出现异常"}, writer)
		return
	}
	if file, err = os.Open(fileInfo.Path); err != nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]读取文件出现异常"}, writer)
		return
	}
	defer file.Close()
	if file == nil {
		ReturnJson(ResMsg{Code: 500, Data: nil, Msg: "[error]文件不存在"}, writer)
		return
	}
	// 设置输出流类型
	writer.Header().Set("Content-Type", "application/octet-stream")
	writer.Header().Set("Content-Disposition", "attachment; filename=\""+url.QueryEscape(fileInfo.Name)+"\"")
	writer.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size, 10))
	// 下载文件，默认限速255KB/s
	bucket := ratelimit.New(defaultRate)
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

// 拷贝项目中的配置文件到指定位置
func CreateExampleConfig() {
	var (
		err               error
		exampleConfigFile *os.File
		configFile        *os.File
	)
	if _, err = os.Stat(ConfigPath + ConfigFileName); err == nil {
		return
	}
	if !utils.FileExists(ConfigPath) { // 是否需要创建文件夹
		_ = os.MkdirAll(ConfigPath, 0775)
	}
	if exampleConfigFile, err = os.Open("./config/config.example.toml"); err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
	defer exampleConfigFile.Close()
	if configFile, err = os.Create(ConfigPath + ConfigFileName); err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
	defer configFile.Close()
	buffer := make([]byte, 1024)
	if _, err = io.CopyBuffer(configFile, exampleConfigFile, buffer); err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
}

// 读取配置文件
func SyncConfig() {
	var (
		err error
	)
	// 拷贝当前目录的配置文件到指定目录
	CreateExampleConfig()
	// 解析，读取配置文件内容
	config, err = toml.LoadFile(ConfigPath + ConfigFileName)
	if err != nil {
		fmt.Println("TomlError ", err.Error())
		os.Exit(1)
		return
	}
	addr = config.Get("app.addr").(string)
	defaultRate = config.Get("app.defaultRate").(int64)
	defaultMaxFileSize = config.Get("app.defaultMaxFileSize").(int64)
	rename = config.Get("file.rename").(bool)
	return
}

func main() {
	// 读取配置文件
	SyncConfig()
	// 初始化数据库
	dbConn, _ = leveldb.OpenFile(FileIndexPath, nil)
	defer dbConn.Close()
	// 初始化HTTP连接
	HandleFunc("/oss/upload.html", UploadHtmlHandler)
	HandleFunc("/oss/api/v1/upload", UploadHandler)
	HandleFunc("/oss/api/v1/download/", DownloadHandler)
	_ = ListenAndServe(addr, nil)
}
