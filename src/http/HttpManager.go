package http

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var HTTPClient *http.Client

func init() {
	var err error
	HTTPClient, err = createHTTPClient()
	if err != nil {
		log.Fatalf("初始化HTTP客户端失败: %v", err)
	}
}

func createHTTPClient() (*http.Client, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		rootCAs = x509.NewCertPool()
	}

	customCert := `
-----BEGIN CERTIFICATE-----
MIIDpzCCAo+gAwIBAgIUIiqEObiDkkfKoRM4yGkMBXh3sV8wDQYJKoZIhvcNAQEL
BQAwYzELMAkGA1UEBhMCQ04xIjAgBgNVBAoMGURyIFByaXZhdGUgVHJ1c3QgU2Vy
dmljZXMxFTATBgNVBAMMDERQVFMgUm9vdCBDQTEZMBcGCSqGSIb3DQEJARYKZHJA
ZGVyLmtpbTAeFw0yNDExMzAxNDMwMzBaFw0zNDExMjgxNDMwMzBaMGMxCzAJBgNV
BAYTAkNOMSIwIAYDVQQKDBlEciBQcml2YXRlIFRydXN0IFNlcnZpY2VzMRUwEwYD
VQQDDAxEUFRTIFJvb3QgQ0ExGTAXBgkqhkiG9w0BCQEWCmRyQGRlci5raW0wggEi
MA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCsJ5arrZvLuO+9vNQnlKOT1KrN
0wh10ntiD7+L1sbRwX8VtbVrhzFMf6IcKVwhfYSeB2UU3xRzy/nORU8TKqbD7QzR
Bgk0rEn/fdfTlcNahjBudpy1mJpCrWjP5Gx6O6Mt64oaoF4kfAzUaizVAJG7zH6E
dnxgbvEcpkm905GUBGrPJ7PWpfRrfQsNHd8ya8FoKM6ceaD3e+NHFgvmFwY2rM09
TV8BZVSrV1rPGJlGMg1bjDHKIBk554kUL2GSukXTChbMfjP7geHcNccsCSplK2ck
pk5B2FS3nMNzdg0CngsqeHKOeI6o3xKzhJmF6+4QDMNhR3hp78DVhciifbRhAgMB
AAGjUzBRMB0GA1UdDgQWBBQRfAsu/OvdvT5wtJqTCElYEYwBtDAfBgNVHSMEGDAW
gBQRfAsu/OvdvT5wtJqTCElYEYwBtDAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3
DQEBCwUAA4IBAQAOgJk/KW8W5Zx96KxcYXdWsyuFwuHv3j2H/+D24NupQLDY5RGh
mBmspG0fkFB+ZsGY1tV/Nl0iWwIIJcM27fc0rahnMvVQ+3mGH2oNxfQlThFSkty3
2Pd16W8aZFAL/Ha4kyzgfdKmzT4vfquLSjZKuzNBTwkQDcFz7xGZir5lRbzCA1YO
mphj7R4G6FwtzNBs9R21tFRzezh6vJr9byZk5oSrqZvckDCHFTa7dC0eWjGVM5la
9fZE6o1HrF89i78lz9O3PZ5vqbza/Ik9TP2XtDJrHcLD5BCjUj7RDnLqBNQB+yR9
DwBWL/y0fMNNNcg8UwtnjmGzip6REXycyFO1
-----END CERTIFICATE-----`

	if ok := rootCAs.AppendCertsFromPEM([]byte(customCert)); !ok {
		return nil, fmt.Errorf("无法添加自定义根证书到证书池")
	}

	tlsConfig := &tls.Config{
		RootCAs: rootCAs,
	}

	tr := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false,
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}, nil
}

const (
	ContentTypeJSON           = "application/json"
	ContentTypeFormURLEncoded = "application/x-www-form-urlencoded"
	ContentTypeTextPlain      = "text/plain"
)

func GetRequest(requestURL string, params map[string]string, headers map[string]string) ([]byte, error) {
	query := url.Values{}
	for k, v := range params {
		query.Add(k, v)
	}

	fullURL := requestURL
	if len(params) > 0 {
		fullURL = requestURL + "?" + query.Encode()
	}

	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	setHeaders(req, headers)
	return doRequest(req)
}

func PostRequest(requestURL string, data interface{}, contentType string, headers map[string]string) ([]byte, error) {
	var body io.Reader
	var err error

	switch contentType {
	case ContentTypeJSON:
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("JSON编码失败: %v", err)
		}
		body = bytes.NewBuffer(jsonData)

	case ContentTypeFormURLEncoded:
		formData, ok := convertToMapStringString(data)
		if !ok {
			return nil, fmt.Errorf("表单数据必须是map[string]string或可转换的类型")
		}
		form := url.Values{}
		for k, v := range formData {
			form.Add(k, v)
		}
		body = strings.NewReader(form.Encode())

	case ContentTypeTextPlain:
		strData, ok := data.(string)
		if !ok {
			return nil, fmt.Errorf("文本数据必须是string类型")
		}
		body = strings.NewReader(strData)

	default:
		return nil, fmt.Errorf("不支持的contentType: %s", contentType)
	}

	req, err := http.NewRequest("POST", requestURL, body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	setHeaders(req, headers)
	req.Header.Set("Content-Type", contentType)
	return doRequest(req)
}

func convertToMapStringString(data interface{}) (map[string]string, bool) {
	switch v := data.(type) {
	case map[string]string:
		return v, true
	case map[string]interface{}:
		result := make(map[string]string)
		for key, val := range v {
			result[key] = fmt.Sprintf("%v", val)
		}
		return result, true
	default:
		return nil, false
	}
}

func setHeaders(req *http.Request, headers map[string]string) {
	if headers != nil {
		for k, v := range headers {
			req.Header.Set(k, v)
		}
	}
}

func doRequest(req *http.Request) ([]byte, error) {
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("请求返回错误状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func Test() {
	getResp, err := GetRequest("https://httpbin.org/get", map[string]string{
		"key1": "value1",
		"key2": "value2",
	}, map[string]string{
		"User-Agent": "MyGoClient",
	})
	if err != nil {
		fmt.Printf("GET请求错误: %v\n", err)
	} else {
		fmt.Printf("GET响应: %s\n", getResp)
	}

	postJSONResp, err := PostRequest("https://httpbin.org/post",
		map[string]interface{}{
			"name":  "John Doe",
			"email": "john@example.com",
		},
		ContentTypeJSON,
		map[string]string{
			"Authorization": "Bearer token123",
		})
	if err != nil {
		fmt.Printf("POST JSON请求错误: %v\n", err)
	} else {
		fmt.Printf("POST JSON响应: %s\n", postJSONResp)
	}

	postFormResp, err := PostRequest("https://httpbin.org/post",
		map[string]string{
			"username": "johndoe",
			"password": "secret",
		},
		ContentTypeFormURLEncoded,
		nil)
	if err != nil {
		fmt.Printf("POST表单请求错误: %v\n", err)
	} else {
		fmt.Printf("POST表单响应: %s\n", postFormResp)
	}

	postRawResp, err := PostRequest("https://httpbin.org/post",
		"name=John&age=30",
		ContentTypeTextPlain,
		nil)
	if err != nil {
		fmt.Printf("POST原始数据请求错误: %v\n", err)
	} else {
		fmt.Printf("POST原始数据响应: %s\n", postRawResp)
	}
}
