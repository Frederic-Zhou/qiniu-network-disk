# 七牛云存储网盘工具

这是一个文件同步工具，基于“七牛云存储”。

This is a tool use for sync local file to Qiniu cloud storage.

## 安装

1. 安装依赖 Dependencies
    - go语言开发 Golang environment
    - sqlite3运行环境 Sqlite3 runtime environment
    - 下载七牛Go SDK : `go get -u github.com/qiniu/api.v7` Qiniu cloud storage Go SDK

2. 编译 Compile

    `go build`

3. 配置 Config

    注册七牛云存储账户，创建私密对象存储空间，并且修改cfg.json文件的字段项

    Register [Qiniu cloud storage account](https://www.qiniu.com), create private object storage space, and modify the item of `cfg.json`.

```
{
    "accessKey": "your accesskey",
    "secretKey": "your secretkey",
    "bucket": "your bucketname",
    "syncFolder": "folder for sync",
    "duration": 10,
    "domain": "your space domain , include 'http://'",
    "zone": "your space store zone"
}
```

- accessKey、secretKey:你的账号下的激活状态的授权信息/
- bucket:存储空间名称
- syncFolder:本地的需要同步的目录路径，可以是相对路径
- duration:文件的扫面周期(秒)
- domain:空间域名，需要带上协议，如：“http://p9xnzre9w.bkt.clouddn.com”
- zone:存储空间所在区域，如：“华南”

4. 运行 Run

    运行go build 后生成的可执行文件

## todo

    本项目目前还只是粗糙的阶段，还有许多未完成的任务。The project is still very rudimentary and there are many unfinished tasks.

目前可以预计的有：

1. 图形化配置界面
2. 自动依赖安装，主要针对sqlite的运行环境，普通用户直接下载已编译的执行文件，所以不需要考虑go语言环境和七牛sdk的安装
3. 文件同步冲突
4. 性能问题