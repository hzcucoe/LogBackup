package ossClinet

import (
	"errors"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type OssConfig struct {
	Endpoint        string
	AccessKeyId     string
	AccessKeySecret string
	Bucket          string
	Dir             string
}

func NewBucket(ossConfig OssConfig) (*oss.Bucket, error) {
	client, err := oss.New(ossConfig.Endpoint, ossConfig.AccessKeyId, ossConfig.AccessKeySecret)
	if err != nil {
		return nil, errors.New("client error")
	}
	return client.Bucket(ossConfig.Bucket)
}
