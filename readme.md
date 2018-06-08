# 七牛云存储网盘工具

这是一个文件同步工具，基于“七牛云存储”。

# 安装

1. 安装依赖：
    - go语言开发
    - sqlite3运行环境
    - 下载七牛Go SDK : `go get -u github.com/qiniu/api.v7`

2. 编译
`go build` 

3. 配置
注册七牛云存储账户，创建私密对象存储空间，并且修改cfg.json文件的字段项
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

- accessKey、secretKey:你的账号下的激活状态的授权信息
- bucket:存储空间名称
- syncFolder:本地的需要同步的目录路径，可以是相对路径
- duration:文件的扫面周期(秒)
- domain:空间域名，需要带上协议，如：“http://p9xnzre9w.bkt.clouddn.com”
- zone:存储空间所在区域，如：“华南”

4. 运行
刚才 go build 后生成的可执行文件


## todo

本项目目前还只是粗糙的阶段，还有许多未完成的任务。

目前可以预计的有：
1. 图形化配置界面
2. 自动依赖安装，主要针对sqlite的运行环境，普通用户直接下载已编译的执行文件，所以不需要考虑go语言环境和七牛sdk的安装
3. 文件同步冲突
4. 性能问题