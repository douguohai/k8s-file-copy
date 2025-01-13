build :
	goreleaser release --snapshot --clean

build-image :
	docker  build -t  registry.cn-beijing.aliyuncs.com/douguohai/k8s-file-copy:2024122001 .