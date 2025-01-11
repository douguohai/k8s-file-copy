/*
copy file to pod
*/
package main

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Pod struct {
	Name          string
	Namespace     string
	ContainerName string
}

// CopyFromPod copies a file or directory from a Pod to the local filesystem.
func (i *Pod) CopyFromPod(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, srcPath string, destPath string) error {
	// Create a pipe to read the tar stream from the Pod
	reader, writer := io.Pipe()
	defer reader.Close()

	// 创建一个 error 通道
	errCh := make(chan error, 1)

	// Start a goroutine to handle the tar stream
	go func() {
		defer writer.Close()
		// 捕获 panic
		if err := i.copyFromPodToTar(ctx, client, config, srcPath, writer); err != nil {
			fmt.Println("Error copying from pod:", err)
			errCh <- fmt.Errorf("error copying from pod: %v", err) // 将错误发送到通道
			return
		}
		errCh <- nil // 没有错误时发送 nil
	}()

	// Extract the tar stream to the local destination
	if err := untarAll(reader, srcPath, destPath); err != nil {
		return fmt.Errorf("failed to untar file: %v", err)
	}

	// 从通道中接收错误
	if err := <-errCh; err != nil {
		return err
	}

	return nil
}

// copyFromPodToTar executes a tar command in the Pod and streams the output to the writer.
func (i *Pod) copyFromPodToTar(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, srcPath string, writer io.Writer) error {
	// Construct the tar command
	cmd := []string{"tar", "cf", "-", srcPath}

	// Create the exec request
	req := client.CoreV1().RESTClient().
		Post().
		Namespace(i.Namespace).
		Resource("pods").
		Name(i.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: i.ContainerName,
			Command:   cmd,
			Stdin:     false,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	// Create the SPDY executor
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create SPDY executor: %v", err)
	}

	// Stream the command output to the writer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: writer,
		Stderr: os.Stdout, // 标准错误重定向到 bytes.Buffer
		Tty:    false,
	})

	if err != nil {
		return fmt.Errorf("failed to stream command output: %v", err)
	}

	return nil
}

// untarAll extracts a tar stream to the local filesystem, preserving only the deepest directory level.
func untarAll(reader io.Reader, srcPath, destPath string) error {
	tarReader := tar.NewReader(reader)
	for {
		// 读取 tar 文件头
		header, err := tarReader.Next()
		if err == io.EOF {
			break // 结束读取
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %v", err)
		}

		// 清理路径并移除 srcPath 前缀
		fileName := strings.TrimPrefix(header.Name, srcPath)
		fileName = strings.TrimPrefix(fileName, "/") // 移除开头的斜杠
		if fileName == "" {
			continue // 如果文件名为空，跳过
		}
		srcDir := filepath.Base(srcPath)
		srcPathPre := strings.Replace(srcPath, srcDir, "", -1)

		// 只保留最底层目录
		fileName = strings.Replace(fmt.Sprintf("/%s", fileName), srcPathPre, "", -1)

		// 构造目标路径
		target := filepath.Join(destPath, fileName)

		fmt.Println(target)

		// 处理目录
		if header.FileInfo().IsDir() {
			if err := os.MkdirAll(target, os.ModePerm); err != nil {
				return fmt.Errorf("failed to create directory: %v", err)
			}
			continue
		}

		// 确保目标文件的父目录存在
		if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
			return fmt.Errorf("failed to create parent directory: %v", err)
		}

		// 如果目标文件已存在，删除它
		if _, err := os.Stat(target); err == nil {
			if err := os.Remove(target); err != nil {
				return fmt.Errorf("failed to remove existing file: %v", err)
			}
		}

		// 创建目标文件
		file, err := os.Create(target)
		if err != nil {
			return fmt.Errorf("failed to create file: %v", err)
		}
		defer file.Close()

		// 将 tar 流写入目标文件
		if _, err := io.Copy(file, tarReader); err != nil {
			return fmt.Errorf("failed to write file: %v", err)
		}
	}

	return nil
}

func (i *Pod) CopyToPod(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, srcPath string, destPath string) error {
	reader, writer := io.Pipe()

	if destPath != "/" && strings.HasSuffix(string(destPath[len(destPath)-1]), "/") {
		destPath = destPath[:len(destPath)-1]
	}

	if err := checkDestinationIsDir(ctx, client, config, i, destPath); err == nil {
		destPath = destPath + "/" + path.Base(srcPath)
	}

	go func() {
		defer writer.Close()
		err := makeTar(srcPath, destPath, writer)
		if err != nil {
			fmt.Println(err)
		}
	}()

	var cmdArr []string

	cmdArr = []string{"tar", "-xf", "-"}
	destDir := path.Dir(destPath)
	if len(destDir) > 0 {
		cmdArr = append(cmdArr, "-C", destDir)
	}
	//remote shell.
	req := client.CoreV1().RESTClient().
		Post().
		Namespace(i.Namespace).
		Resource("pods").
		Name(i.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: i.ContainerName,
			Command:   cmdArr,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  reader,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})
	if err != nil {
		return err
	}
	return nil
}

func checkDestinationIsDir(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, i *Pod, destPath string) error {
	return i.Exec(ctx, client, config, []string{"test", "-d", destPath})
}

func makeTar(srcPath, destPath string, writer io.Writer) error {
	// TODO: use compression here?
	tarWriter := tar.NewWriter(writer)
	defer tarWriter.Close()

	srcPath = path.Clean(srcPath)
	destPath = path.Clean(destPath)
	return recursiveTar(path.Dir(srcPath), path.Base(srcPath), path.Dir(destPath), path.Base(destPath), tarWriter)
}

func recursiveTar(srcBase, srcFile, destBase, destFile string, tarWriter *tar.Writer) error {

	// defer func() {
	// 	fmt.Println("d")
	// 	if err := recover(); err != nil {
	// 		fmt.Println(err) // 这里的err其实就是panic传入的内容
	// 	}
	// 	fmt.Println("e")
	// }()

	filepath := path.Join(srcBase, srcFile)
	stat, err := os.Lstat(filepath)
	if err != nil {
		return err
	}
	if stat.IsDir() {
		files, err := ioutil.ReadDir(filepath)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			//case empty directory
			hdr, _ := tar.FileInfoHeader(stat, filepath)
			hdr.Name = destFile
			if err := tarWriter.WriteHeader(hdr); err != nil {
				return err
			}
		}
		for _, f := range files {
			if err := recursiveTar(srcBase, path.Join(srcFile, f.Name()), destBase, path.Join(destFile, f.Name()), tarWriter); err != nil {
				return err
			}
		}
		return nil
	} else if stat.Mode()&os.ModeSymlink != 0 {
		//case soft link
		hdr, _ := tar.FileInfoHeader(stat, filepath)
		target, err := os.Readlink(filepath)
		if err != nil {
			return err
		}

		hdr.Linkname = target
		hdr.Name = destFile
		if err := tarWriter.WriteHeader(hdr); err != nil {
			return err
		}
	} else {
		//case regular file or other file type like pipe
		hdr, err := tar.FileInfoHeader(stat, filepath)
		if err != nil {
			return err
		}
		hdr.Name = destFile
		err = tarWriter.WriteHeader(hdr)
		if err != nil {
			log.Println(err)
			return err
		}

		f, err := os.Open(filepath)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(tarWriter, f); err != nil {
			return err
		}
		return f.Close()
	}
	return nil
}

func (i *Pod) Exec(ctx context.Context, client *kubernetes.Clientset, config *rest.Config, cmd []string) error {

	req := client.CoreV1().RESTClient().
		Post().
		Namespace(i.Namespace).
		Resource("pods").
		Name(i.Name).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: i.ContainerName,
			Command:   cmd,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  strings.NewReader(""),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Tty:    false,
	})

	if err != nil {
		return err
	}
	return nil
}
