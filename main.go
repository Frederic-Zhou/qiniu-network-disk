package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/qiniu/api.v7/auth/qbox"
	"github.com/qiniu/api.v7/storage"
	"golang.org/x/net/context"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
		syncDownFile(db) //同步远端有本地无的文件
		syncUpFile(db)   //同步本端有远端无的文件,删除本段有远端无的文件
		fmt.Println("syncUpFile over")
		time.Sleep(time.Second * config.Duration)
	}

}

func syncUpFile(db *sql.DB) {

	//将所有文件清单中的状态设置为0
	db.Exec("UPDATE files SET exist=0 WHERE downloading=0")

	//获取todo文件夹中的文件
	localFiles := getLocalFiles(config.SyncFolder)

	for _, f := range localFiles {

		modTime, size, count, downloading, err := getFileStoreInfo(db, f.PATH)
		if err != nil {
			fmt.Println("getFileStoreInfo: ", err.Error())
			continue
		}

		fmt.Printf("name:%s ; count:%d ; modTime: %s ;size:%d ", f.PATH, count, f.ModTime.Format("2006/01/02@15:04:05"), f.Size)

		if count == 0 { //文件不存在于文件清单中

			if _, err = upload(f.PATH, f.PATH); err != nil {
				log.Println("上传错误", err.Error())
				continue
			}

			if _, err = db.Exec("INSERT INTO files (fileName,modTime,size,exist) VALUES (?,?,?,1)", f.PATH, f.ModTime, f.Size); err != nil {
				log.Println("更新数据错误", err.Error())
			}

			fmt.Println("添加到数据库中")

		} else if (modTime.UnixNano() != f.ModTime.UnixNano() || size != f.Size) && downloading == 0 { //文件修改时间或尺寸不一致

			if _, err = upload(f.PATH, f.PATH); err != nil {
				log.Println("上传错误", err.Error())
				continue
			}

			if _, err = db.Exec("UPDATE files SET modTime=?,size=?,exist=1 WHERE fileName=?", f.ModTime, f.Size, f.PATH); err != nil {
				log.Println("更新数据错误", err.Error())
			}

			fmt.Println("更新到数据库中")
		} else {
			//将文件存在状态改为1
			db.Exec("UPDATE files SET exist=1 WHERE fileName=?", f.PATH)

			fmt.Println("最新，无需同步")
			continue
		}

	}

	//删除所有 exist=0的文件
	delNotExistFiles(db)
}

func syncDownFile(db *sql.DB) {
	remoteFiles := getRemoteFileList()

	for _, rf := range remoteFiles {
		_, _, count, _, err := getFileStoreInfo(db, rf.Key)
		if err != nil {
			fmt.Println("download getFileStoreInfo", err.Error())
			continue
		}

		if count == 0 { //清单中不存在此文件，

			//记录到文件清单中
			if _, err = db.Exec("INSERT INTO files (fileName,modTime,exist,downloading) VALUES (?,?,1,1)", rf.Key, time.Now()); err != nil {
				log.Println("更新数据错误", err.Error())
				continue
			}

			fileInfo, err := downloadFile(rf.Key)
			if err != nil {
				//如果下载文件出错，回退文件清单
				db.Exec("DELETE FROM files WHERE fileName=?", rf.Key)
			}

			//更新到文件清单中
			if _, err = db.Exec("UPDATE files SET downloading=0,modTime=?,size=? WHERE fileName=?", fileInfo.ModTime(), fileInfo.Size(), rf.Key); err != nil {
				log.Println("更新数据错误", err.Error())
			}

			fmt.Println("文件保存完成")

		}
	}
}

func downloadFile(fileName string) (fileInfo os.FileInfo, err error) {
	//下载到本地，并且记录到清单中
	fmt.Println("下载文件...", fileName)
	//获取URL，下载
	dURL := getDownloadURL(fileName)
	res, err := http.Get(dURL)
	if err != nil {
		log.Println("download error", err.Error())
		return
	}

	//创建目录
	if err = os.MkdirAll(filepath.Dir(fileName), 0755); err != nil {
		log.Println("create folder error", err.Error())
		return
	}

	//创建文件
	file, err := os.Create(fileName)
	if err != nil {
		log.Println("create file error", err.Error())
		return
	}
	defer file.Close()

	//写入到文件
	_, err = io.Copy(file, res.Body)
	if err != nil {
		log.Println("write file error", err.Error())
		return
	}

	return file.Stat()

}

func getFileStoreInfo(db *sql.DB, fileName string) (modTime time.Time, size int64, count, downloading int, err error) {

	rows, err := db.Query("SELECT modTime,size,downloading FROM files WHERE fileName=?", fileName)
	if err != nil {
		return
	}

	for rows.Next() {
		err = rows.Scan(&modTime, &size, &downloading)
		if err != nil {
			return
		}
		count++
	}

	return
}

func delNotExistFiles(db *sql.DB) {

	rows, err := db.Query("SELECT fileName FROM files WHERE exist=0")
	if err != nil {
		log.Println("查询不存在的文件出错:", err.Error())
		return
	}

	var fileName string
	for rows.Next() {

		err = rows.Scan(&fileName)
		if err != nil {
			log.Println("读取数据文件出错:", err.Error())
			return
		}

		fmt.Println("file will del", fileName)
		//删除远端文件
		if err = delRemoteFile(fileName); err != nil {
			log.Println("删除远端文件出错：", err.Error())
		}

	}

	_, err = db.Exec("DELETE FROM files WHERE exist=0")
	if err != nil {
		log.Println("删除文件清单报错：", err.Error())
		return
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
		size INTEGER DEFAULT 0,
		exist INTEGER DEFAULT 0,
		downloading INTEGER DEFAULT 0
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

func getLocalFiles(folder string) (fs []FileInfo) {
	files, _ := ioutil.ReadDir(folder)
	for _, file := range files {

		if file.IsDir() {
			fs = append(fs, getLocalFiles(folder+"/"+file.Name())...)
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
	Zone           string        `json:"zone"`
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
		Scope: fmt.Sprintf("%s:%s", config.Bucket, key),
		//FileType: 1, //1 低频存储
	}
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)
	upToken := putPolicy.UploadToken(mac)

	cfg := storage.Config{}
	// 空间对应的机房

	switch config.Zone {
	case "华东":
		cfg.Zone = &storage.ZoneHuadong
	case "华北":
		cfg.Zone = &storage.ZoneHuabei
	case "华南":
		cfg.Zone = &storage.ZoneHuanan
	case "北美":
		cfg.Zone = &storage.ZoneBeimei
	}

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
	downloadURL = "http://" + downloadURL
	return
}

func getRemoteFileList() (remoteFiles []storage.ListItem) {
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)
	cfg := storage.Config{
		// 是否使用https域名进行资源管理
		UseHTTPS: false,
	}
	bucketManager := storage.NewBucketManager(mac, &cfg)
	limit := 1000
	prefix := config.SyncFolder
	delimiter := ""
	//初始列举marker为空
	marker := ""
	for {
		entries, _, nextMarker, hashNext, err := bucketManager.ListFiles(config.Bucket, prefix, delimiter, marker, limit)
		if err != nil {
			fmt.Println("list error,", err)
			break
		}

		remoteFiles = append(remoteFiles, entries...)

		if hashNext {
			marker = nextMarker
		} else {
			//list end
			break
		}
	}

	return
}

func delRemoteFile(fileName string) (err error) {
	mac := qbox.NewMac(config.AccessKey, config.SecretKey)
	cfg := storage.Config{
		// 是否使用https域名进行资源管理
		UseHTTPS: false,
	}
	bucketManager := storage.NewBucketManager(mac, &cfg)

	return bucketManager.Delete(config.Bucket, fileName)
}
