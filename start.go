package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"io"
	"k3shelp/base"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var fileTempDir = "./temp"

var (
	clientSet *kubernetes.Clientset
	config    *restclient.Config
	err       error
	Cron      = cron.New()
)

func init() {
	// 1. 加载 kubeConfig 文件
	kubeConfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		panic(err.Error())
	}

	// 2. 创建 Kubernetes 客户端
	clientSet, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

}

func main() {

	// 记录到文件。
	f, _ := os.Create("gin.log")
	gin.DefaultWriter = io.MultiWriter(f)

	// 如果需要同时将日志写入文件和控制台，请使用以下代码。
	gin.DefaultWriter = io.MultiWriter(f, os.Stdout)

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	// 绑定 JSON ({"user": "manu", "password": "123"})
	r.POST("/copy/local/2/pod", func(c *gin.Context) {
		var copy2pod base.Copy2Pod
		if err := c.ShouldBindJSON(&copy2pod); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("copy2pod 接收参数:%s", copy2pod.ToJSONString())
		//下载文件至本地空间
		err, filePath := base.DownloadFileFromMinIO(copy2pod.SourceFileUrl, fileTempDir)
		//移动本地缓存文件至相关pod内
		if err != nil {
			log.Printf("copy2pod 文件下载失败：%s", copy2pod.SourceFileUrl)
			c.JSON(http.StatusOK, gin.H{"code": "500", "message": "文件下载失败" + err.Error()})
			return
		}

		log.Printf("copy2pod 文件缓存本地目录:%s", filePath)

		// 3. 获取 Deployment 对应的 Pod
		pods, err := clientSet.CoreV1().Pods(copy2pod.TargetNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("run=%s", copy2pod.TargetDeployment), // 假设 Deployment 的标签是 app=<deployment-name>
		})

		if err != nil {
			log.Printf("查询 k8s集群失败:%s ", copy2pod.ToJSONString())
			c.JSON(http.StatusOK, gin.H{"code": "500", "message": "操作失败,没定位到pod的" + err.Error()})
			return
		}

		if len(pods.Items) == 0 {
			log.Printf("copy2pod k8s 根据 namespace: %s 和 deploymenmt：%s 没有找到存活的pod ", copy2pod.TargetNamespace, copy2pod.TargetDeployment)
			c.JSON(http.StatusOK, gin.H{"code": "500", "message": "操作失败,没定位到pod的" + err.Error()})
			return
		}

		podName := pods.Items[0].Name                          // 获取第一个 Pod 的名称
		containerName := pods.Items[0].Spec.Containers[0].Name // 获取第一个容器的名称
		log.Printf("Found pod: %s, container: %s\n", podName, containerName)

		pod := Pod{
			Name:          podName,
			Namespace:     copy2pod.TargetNamespace,
			ContainerName: containerName,
		}

		err = pod.CopyToPod(context.Background(), clientSet, config, filePath, copy2pod.TargetDir)
		if err != nil {
			log.Printf("复制文件失败: %s", copy2pod.ToJSONString())
			c.JSON(http.StatusOK, gin.H{"code": "500", "message": "复制文件失败" + err.Error()})
			return
		}
		log.Printf("复制文件成功: %s", copy2pod.ToJSONString())
		c.JSON(http.StatusOK, gin.H{"code": "0", "message": "操作成功"})
	})

	// 绑定 JSON ({"user": "manu", "password": "123"})
	r.POST("/copy/pod/2/local", func(c *gin.Context) {
		var pod2local base.CopyFormPod
		if err := c.ShouldBindJSON(&pod2local); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// 3. 获取 Deployment 对应的 Pod
		pods, err := clientSet.CoreV1().Pods(pod2local.TargetNamespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf("run=%s", pod2local.TargetDeployment), // 假设 Deployment 的标签是 app=<deployment-name>
		})

		if err != nil {
			log.Printf("查询 k8s集群失败:%s ", pod2local.ToJSONString())
			c.JSON(http.StatusOK, gin.H{"code": "500", "message": "操作失败,没定位到pod的" + err.Error()})
			return
		}

		if len(pods.Items) == 0 {
			log.Printf("pod2local k8s 根据 namespace: %s 和 deploymenmt：%s 没有找到存活的pod ", pod2local.TargetNamespace, pod2local.TargetDeployment)
			c.JSON(http.StatusOK, gin.H{"code": "500", "message": "操作失败,没定位到pod的" + err.Error()})
			return
		}

		podName := pods.Items[0].Name                          // 获取第一个 Pod 的名称
		containerName := pods.Items[0].Spec.Containers[0].Name // 获取第一个容器的名称
		log.Printf("pod2local Found pod: %s, container: %s\n", podName, containerName)

		pod := Pod{
			Name:          podName,
			Namespace:     pod2local.TargetNamespace,
			ContainerName: containerName,
		}

		err = pod.CopyFromPod(context.Background(), clientSet, config, pod2local.TargetFile, fileTempDir)
		if err != nil {
			log.Printf("pod2local 复制文件失败: %s", pod2local.ToJSONString())
			c.JSON(http.StatusOK, gin.H{"code": "500", "message": "复制文件失败" + err.Error()})
			return
		}
		log.Printf("pod2local 复制文件成功: %s", pod2local.ToJSONString())
		c.JSON(http.StatusOK, gin.H{"code": "0", "message": "操作成功"})
	})

	r.GET("/static/*filepath", customStatic)

	Cron.Start()

	_, err := Cron.AddFunc("0 2 * * *", cleanTempDir) // 每隔1分钟执行一次DeleteStaffs方法
	if err != nil {
		log.Printf("定时清理临时文件失败。。。")
		return
	}

	r.Run() // 监听并在 0.0.0.0:8080 上启动服务
}

func cleanTempDir() {
	log.Println("执行定时任务")
	// 这里可以添加具体的任务逻辑
	dir, err := os.Stat(fileTempDir)
	if err == nil {
		if dir.IsDir() {
			_ = os.RemoveAll(fileTempDir)
		}
	}
}

func customStatic(c *gin.Context) {
	// 获取请求的路径
	requestPath := c.Request.URL.Path
	// 假设静态文件的根目录是 "static"，你可以根据实际情况修改
	staticRoot := "temp"
	// 拼接完整的文件路径
	filePath := path.Join(staticRoot, strings.TrimPrefix(requestPath, "/static"))
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件未找到"})
		return
	}
	if fileInfo.IsDir() {
		c.JSON(http.StatusForbidden, gin.H{"error": "禁止访问目录列表"})
		return
	}
	c.File(filePath)
}
