package base

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func getFileNameFromMinioPreviewUrl(previewUrl string) string {
	// 解析URL
	parsedUrl, err := url.Parse(previewUrl)
	if err != nil {
		fmt.Printf("解析URL失败: %v\n", err)
		return ""
	}
	// 获取URL路径部分
	path := parsedUrl.Path
	// 通过路径分隔符来拆分路径，获取最后一部分，通常就是文件名
	pathSegments := strings.Split(path, "/")
	if len(pathSegments) == 0 {
		return ""
	}
	return pathSegments[len(pathSegments)-1]
}

// DownloadFileFromMinIO 从 MinIO 的预签名 URL 下载文件并保存到本地
func DownloadFileFromMinIO(url string, localDir string) (error, string) {
	// 1. 发送 HTTP GET 请求下载文件
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download file: %v", err), ""
	}
	defer resp.Body.Close()

	// 2. 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: status code %d", resp.StatusCode), ""
	}

	// 3. 创建本地目录（如果不存在）
	if err := os.MkdirAll(localDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err), ""
	}

	// 4. 生成本地文件路径
	localFilePath := filepath.Join(localDir, getFileNameFromMinioPreviewUrl(url))
	// 5. 创建本地文件
	file, err := os.Create(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err), ""
	}
	defer file.Close()

	// 6. 将下载的文件内容写入本地文件
	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("failed to write file content: %v", err), ""
	}

	fmt.Printf("File downloaded successfully: %s\n", localFilePath)
	return nil, localFilePath
}
