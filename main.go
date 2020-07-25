package main

import (
	"LogBackup/log"
	ossClinet "LogBackup/oss"
	"archive/zip"
	"fmt"
	"github.com/CatchZeng/dingtalk/client"
	"github.com/CatchZeng/dingtalk/message"
	"github.com/jinzhu/configor"
	"github.com/robfig/cron/v3"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var Config = struct {
	UploadDriver string `default:"oss"`

	OSS ossClinet.OssConfig

	Backups []struct {
		Name string
		Path string `required:"true"`
	}

	BackupDays string

	Dingding DingDing

}{}

type DingDing struct {
	Enable bool
	AccessToken string
	Secret string
}

var (
	dingTalk client.DingTalk
)

func main() {
	// 日志设置
	log.InitLog()
	log.Log.Info("Starting...")
	// step 1   加载配置文件
	_ = configor.Load(&Config, "./conf/config.yml")
	if Config.Dingding.Enable {
		dingTalk = client.DingTalk{
			AccessToken: Config.Dingding.AccessToken,
			Secret:      Config.Dingding.Secret,
		}
	}
	// 每次启动先备份一次
	handleLogBackup()

	loc, _ := time.LoadLocation("Local")
	// 定义一个cron运行器
	c := cron.New(cron.WithLocation(loc))
	// 定时
	_,err := c.AddFunc("0 0 */"+Config.BackupDays+" * *", handleLogBackup)
	if err!=nil {
		panic(err.Error())
	}
	// 开始
	c.Start()
	defer c.Stop()
	select {
	}
}

// 打包成zip文件
func Zip(srcDir string, zipFileName string)  error {

	// 预防：旧文件无法覆盖
	_ = os.RemoveAll(zipFileName)

	// 创建：zip文件
	zipFile, err := os.Create(zipFileName)
	defer func() {
		_ = zipFile.Close()
	}()
	if err !=nil {
		return err
	}

	// 打开：zip文件
	archive := zip.NewWriter(zipFile)
	defer func() {_=archive.Close()}()

	// 遍历路径信息
	err=filepath.Walk(srcDir, func(path string, info os.FileInfo, _ error) error {

		// 如果是源路径，提前进行下一个遍历
		if path == srcDir {
			return nil
		}

		// 获取：文件头信息
		header, _ := zip.FileInfoHeader(info)
		header.Name = strings.TrimPrefix(path, srcDir)
		// 判断：文件是不是文件夹
		if info.IsDir() {
			header.Name += `/`
		} else {
			// 设置：zip的文件压缩算法
			header.Method = zip.Deflate
		}

		// 创建：压缩包头部信息
		writer, _ := archive.CreateHeader(header)
		if !info.IsDir() {
			file, _ := os.Open(path)
			defer func() {_=file.Close()}()
			_,_=io.Copy(writer, file)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func handleLogBackup()  {
	bucket,err := ossClinet.NewBucket(Config.OSS)
	if err != nil || bucket == nil {
		// TODO 邮件
		msg := message.NewTextMessage().SetContent("bucket 获取失败")
		_,_=dingTalk.Send(msg)
		log.Log.Error("bucket 获取失败")
		return
	}

	// step 2   读取需要备份的目录
	backups := Config.Backups
	errBackUps := make([]string,0)
	for i := 0; i < len(backups); i++ {
		// step 3   备份的目录打包
		path := backups[i].Path
		if path == "" {
			continue
		}
		ZipName := backups[i].Name + "-"+time.Now().Format("2006-01-02")+".zip"
		err := Zip(path,ZipName)
		defer func() {
			// 删除压缩文件
			err=os.Remove("./"+ZipName)
			if err != nil {
				log.Log.Error("删除压缩文件 "+ZipName+" 失败")
			}
		}()
		if err != nil {
			errBackUps = append(errBackUps,backups[i].Path )
			log.Log.Error("压缩文件 "+ZipName+" 失败")
			continue
		}
		// step 4   打包文件上传
		fmt.Println(Config.OSS.Dir+"/"+ZipName)
		err = bucket.PutObjectFromFile(Config.OSS.Dir+"/"+ZipName,ZipName)
		if err != nil {
			errBackUps = append(errBackUps,backups[i].Path )
			log.Log.Error("压缩文件 "+ZipName+" 上传OSS失败:"+err.Error())
			continue
		}

	}
	totalBackUpNum := len(backups)
	msgText := "总共备份目录"+strconv.Itoa(totalBackUpNum)+"个，失败 "+strconv.Itoa(len(errBackUps))+"个"
	if len(errBackUps) > 0 {
		msgText += "，失败明细如下："+strings.Join(errBackUps,"\n")
	}
	// step 5   提醒(钉钉，邮件)
	if Config.Dingding.Enable {
		msg := message.NewTextMessage().SetContent(msgText)
		_,_=dingTalk.Send(msg)
	}
	log.Log.Info(msgText)
}