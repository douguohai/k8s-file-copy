package base

import "encoding/json"

// Copy2Pod 目录拷贝请求
type Copy2Pod struct {
	SourceFileUrl    string `json:"sourceFileUrl" binding:"required"`
	TargetNamespace  string `json:"targetNamespace" binding:"required"`
	TargetDeployment string `json:"targetDeployment" binding:"required"`
	TargetDir        string `json:"targetDir" binding:"required"`
}

// ToJSONString 转json字符串
func (o Copy2Pod) ToJSONString() string {
	j, _ := json.Marshal(o)
	return string(j)
}

type Result struct {
	Code       interface{} `json:"errorCode"`
	ErrMessage interface{} `json:"errorMessage"`
}

func (r Result) ToJSONStr() string {
	jsonBytes, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(jsonBytes)
}

// CopyFormPod 目录拷贝请求
type CopyFormPod struct {
	TargetNamespace  string `json:"targetNamespace" binding:"required"`
	TargetDeployment string `json:"targetDeployment" binding:"required"`
	TargetFile       string `json:"targetFile" binding:"required"`
}

// ToJSONString 转json字符串
func (o CopyFormPod) ToJSONString() string {
	j, _ := json.Marshal(o)
	return string(j)
}
