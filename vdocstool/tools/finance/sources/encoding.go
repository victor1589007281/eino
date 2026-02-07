// Package sources GBK编码转换
package sources

import (
	"bytes"
	"io"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// GBKToUTF8 将GBK编码转换为UTF-8
func GBKToUTF8(data []byte) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
	result, err := io.ReadAll(reader)
	if err != nil {
		return data, err // 转换失败返回原数据
	}
	return result, nil
}

// UTF8ToGBK 将UTF-8编码转换为GBK
func UTF8ToGBK(data []byte) ([]byte, error) {
	reader := transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewEncoder())
	result, err := io.ReadAll(reader)
	if err != nil {
		return data, err
	}
	return result, nil
}

// GBKToUTF8String 字符串转换
func GBKToUTF8String(s string) string {
	result, err := GBKToUTF8([]byte(s))
	if err != nil {
		return s
	}
	return string(result)
}
