package base

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var src = rand.NewSource(time.Now().UnixNano())

func init() {
	rand.Seed(time.Now().Unix())
}

func createTempAKSK() (accessKeyId string, plainSk string, err error) {
	if accessKeyId, err = generateAccessKeyId("AKTP"); err != nil {
		return
	}

	plainSk, err = generateSecretKey()
	if err != nil {
		return
	}
	return
}

func generateAccessKeyId(prefix string) (string, error) {
	uuid := uuid.New()

	uidBase64 := base64.StdEncoding.EncodeToString([]byte(strings.Replace(uuid.String(), "-", "", -1)))

	s := strings.Replace(uidBase64, "=", "", -1)
	s = strings.Replace(s, "/", "", -1)
	s = strings.Replace(s, "+", "", -1)
	s = strings.Replace(s, "-", "", -1)
	return prefix + s, nil
}

func randStringRunes(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}
	return string(b)
}

func generateSecretKey() (string, error) {
	randString32 := randStringRunes(32)
	return aesEncryptCBCWithBase64([]byte(randString32), []byte("bytedance-isgood"))
}

func createInnerToken(credentials Credentials, sts *SecurityToken2, inlinePolicy *Policy, t int64) (*InnerToken, error) {
	var err error
	innerToken := new(InnerToken)

	innerToken.LTAccessKeyId = credentials.AccessKeyID
	innerToken.AccessKeyId = sts.AccessKeyID
	innerToken.ExpiredTime = t

	key := md5.Sum([]byte(credentials.SecretAccessKey))
	innerToken.SignedSecretAccessKey, err = aesEncryptCBCWithBase64([]byte(sts.SecretAccessKey), key[:])
	if err != nil {
		return nil, err
	}

	if inlinePolicy != nil {
		b, _ := json.Marshal(inlinePolicy)
		innerToken.PolicyString = string(b)
	}

	signStr := fmt.Sprintf("%s|%s|%d|%s|%s", innerToken.LTAccessKeyId, innerToken.AccessKeyId, innerToken.ExpiredTime, innerToken.SignedSecretAccessKey, innerToken.PolicyString)

	innerToken.Signature = hex.EncodeToString(hmacSHA256(key[:], signStr))
	return innerToken, nil
}

func getTimeout(serviceTimeout, apiTimeout time.Duration) time.Duration {
	timeout := time.Second
	if serviceTimeout != time.Duration(0) {
		timeout = serviceTimeout
	}
	if apiTimeout != time.Duration(0) {
		timeout = apiTimeout
	}
	return timeout
}

func mergeQuery(query1, query2 url.Values) (query url.Values) {
	query = url.Values{}
	if query1 != nil {
		for k, vv := range query1 {
			for _, v := range vv {
				query.Add(k, v)
			}
		}
	}

	if query2 != nil {
		for k, vv := range query2 {
			for _, v := range vv {
				query.Add(k, v)
			}
		}
	}
	return
}

func mergeHeader(header1, header2 http.Header) (header http.Header) {
	header = http.Header{}
	if header1 != nil {
		for k, v := range header1 {
			header.Set(k, strings.Join(v, ";"))
		}
	}
	if header2 != nil {
		for k, v := range header2 {
			header.Set(k, strings.Join(v, ";"))
		}
	}

	return
}

func NewAllowStatement(actions, resources []string) *Statement {
	sts := new(Statement)
	sts.Effect = "Allow"
	sts.Action = actions
	sts.Resource = resources

	return sts
}

func NewDenyStatement(actions, resources []string) *Statement {
	sts := new(Statement)
	sts.Effect = "Deny"
	sts.Action = actions
	sts.Resource = resources

	return sts
}
