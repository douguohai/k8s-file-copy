FROM swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/golang:alpine3.21 AS builder
LABEL authors="douguohai@gmail.com"

WORKDIR /app

ADD . /app

RUN GOPROXY='https://goproxy.cn' GO111MODULE=on go build -o k8s-file-copy


FROM swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/alpine:latest AS runner
LABEL authors="douguohai@gmail.com"
# 设置工作目录
WORKDIR /app

RUN mkdir $HOME/.kube/

# 从构建阶段（builder）拷贝构建好的二进制文件到运行时镜像中
COPY --from=builder /app/k8s-file-copy  /app/file-copy

CMD ["./file-copy "]