package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/qiniu/api.v7/auth/qbox"
	"github.com/qiniu/api.v7/storage"
	"golang.org/x/net/context"
	"io/ioutil"
	"log"
	"net/url"
	"time"
)

var config Config

func main() {
	//读取配置文件，设置常规配置
	if err := loadConfig(); err != nil {
		log.Fatalln("配置文件:", err.Error())
	}

	db, err := createDB()
	if err != nil {
		log.Fatalln("数据库错误", err.Error())
	}

	defer db.Close()

	for {
		syncFile(db)
		fmt.Println("ansyncFile over")
		time.Sleep(time.Second * config.Duration)
	}

}

func syncFile(db *sql.DB) {

	//获取todo文件夹中的文件
	todofiles := listFile(config.SyncFolder)

	for _, f := range todofiles {

		modTime, size, count, err := getFileStoreInfo(db, f.PATH)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		fmt.Printf("name:%s ; count:%d ; modTime: %s ;size:%d ", f.PATH, count, f.ModTime.Format("2006/01/02@15:04:05"), f.Size)

		if count == 0 { //文件不存在

			if _, err = upload(f.PATH, f.PATH); err != nil {
				log.Println("上传错误", err.Error())
				continue
			}

			if _, err = db.Exec("INSERT INTO files (fileName,modTime,size) VALUES (?,?,?)", f.PATH, f.ModTime, f.Size); err != nil {
				log.Println("更新数据错误", err.Error())
			}

			fmt.Println("添加到数据库中")

		} else if modTime.UnixNano() != f.ModTime.UnixNano() || size != f.Size { //文件修改时间或尺寸不一致

			if _, err = upload(f.PATH, f.PATH); err != nil {
				log.Println("上传错误", err.Error())
				continue
			}

			if _, err = db.Exec("UPDATE files SET modTime=?,size=? WHERE fileName=?", f.ModTime, f.Size, f.PATH); err != nil {
				log.Println("更新数据错误", err.Error())
			}

			fmt.Println("更新到数据库中")
		} else {
			fmt.Println("最新，无需同步")
			continue
		}

	}
}

func getFileStoreInfo(db *sql.DB, fileName string) (modTime time.Time, size int64, count int, err error) {
	rows, err := db.Query("SELECT modTime,size FROM files WHERE fileName=?", fileName)
	if err != nil {
		return
	}

	for rows.Next() {
		err = rows.Scan(&modTime, &size)
		if err != nil {
			return
		}
		count++
	}

	return
}

func createDB() (db *sql.DB, err error) {
	db, err = sql.Open("sqlite3", "./.files.db")
	if err != nil {
		return
	}

	//创建表
	createTable := `
    CREATE TABLE IF NOT EXISTS files(
        fileName VARCHAR(255) PRIMARY KEY,
		modTime DATE NULL,
		size INTEGER 
    );
	`

	_, err = db.Exec(createTable)

	return
}

// FileInfo 记录文件的具体类型
type FileInfo struct {
	Name    string
	PATH    string
	ModTime time.Time
	Size    int64
}

func listFile(folder string) (fs []FileInfo) {
	files, _ := ioutil.ReadDir(folder)
	for _, file := range files {

		if file.IsDir() {
			fs = append(fs, listFile(folder+"/"+file.Name())...)
		} else {
			fs = append(fs, FileInfo{Name: file.Name(),
				PATH:    folder + "/" + file.Name(),
				ModTime: file.ModTime(),
				Size:    file.Size(),
			})
		}
	}

	return
}

// Config 配置文件对象
type Config struct {
	AccessKey      string        `json:"accessKey"`
	SecretKey      string        `json:"secretKey"`
	Bucket         string        `json:"bucket"`
	SyncFolder     string        `json:"syncFolder"`
	Duration       time.Duration `json:"duration"`
	DownloadDomain string        `json:"downloadDomain"`
}

func loadConfig() (err error) {

	//读取配置文件
	body, err := ioutil.ReadFile("./cfg.json")
	if err != nil {
		log.Fatalln("没有找到配置文件", "./cfg.json", err.Error())
	}

	err = json.Unmarshal(body, &config)
	if err != nil {
		log.Fatalln("配置文件不是标准的Json", err.Error())
	}

	fmt.Println(config)

	return
}

func upload(key, localFile string) (ret storage.PutRet, err error) {

	putPolicy := storage.PutPolicy{
		Scope:    fmt.Sprintf("%s:%s", config.Bucket, key),
		FileType: 1,
	}
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)
	upToken := putPolicy.UploadToken(mac)

	cfg := storage.Config{}
	// 空间对应的机房
	cfg.Zone = &storage.ZoneHuanan
	// 是否使用https域名
	cfg.UseHTTPS = false
	// 上传是否使用CDN上传加速
	cfg.UseCdnDomains = false
	// 构建表单上传的对象
	formUploader := storage.NewFormUploader(&cfg)

	// 可选配置
	putExtra := storage.PutExtra{}

	err = formUploader.PutFile(context.Background(), &ret, upToken, key, localFile, &putExtra)
	return
}

func getDownloadURL(fileName string) (downloadURL string) {
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)
	key := url.QueryEscape(fileName)
	deadline := time.Now().Add(time.Second * 3600).Unix() //1小时有效期
	downloadURL = storage.MakePrivateURL(mac, config.DownloadDomain, key, deadline)

	return
}
