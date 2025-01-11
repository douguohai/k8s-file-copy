### k8s-file-copy

支持在k8s集群内进行文件复制，需挂在配置文件到 $HOME/.kube/config

注意 targetDeployment 只允许有一个存活的pod，其他场景未考虑到

支持 local2pod

post http://localhost:8080/copy/local/2/pod

```json
{
    "sourceFileUrl":"https://www.baidu.com/alpha/db/other/20241025/fc5f6da213994d549f39325e8597c3ad.png",
    "targetNamespace":"alpha",
    "targetDeployment":"db-pdf-sign",
    "targetDir":"/root/"
}
```

文件将会存在pod的对应目录下，如/root/fc5f6da213994d549f39325e8597c3ad.png
```json
{
    "code": 0,
    "message": "操作成功"
}
```

支持 pod2local

post http://localhost:8080/copy/pod/2/local

```json
{
    "targetNamespace":"alpha",
    "targetDeployment":"db-pdf-sign",
    "targetFile":"/root/fc5f6da213994d549f39325e8597c3ad.png"
}

```

会从指定pod的对应目录下，将文件拷贝到本地。
```json
{
  "code": 0,
  "message": "操作成功",
  "data": {
    "url": "http://localhost:8080/static/fc5f6da213994d549f39325e8597c3ad.png"
  }
}
```