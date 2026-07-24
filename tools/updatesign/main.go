// Command updatesign 提供更新包校验文件的 Ed25519 密钥生成与签名能力。
//
// 子命令：
//
//	genkey
//	    生成一对 Ed25519 密钥，向标准输出打印 base64 编码的私钥与公钥。
//	    私钥为 crypto/ed25519 的 64 字节形式，需配置到 CI 私密变量；
//	    公钥为 32 字节形式，需配置到运行环境 ALLBOT_UPDATE_ED25519_PUBLIC_KEY。
//
//	sign -in <文件> [-out <文件>]
//	    从环境变量 ALLBOT_UPDATE_ED25519_PRIVATE_KEY 读取 base64 私钥，
//	    对输入文件的原始字节做 Ed25519 签名，将 base64 编码的签名写入
//	    -out 指定文件（默认 <文件>.sig）。签名对象与客户端验签保持一致：
//	    客户端验证的 payload 是校验文件的完整原始字节。
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"strings"
)

// privateKeyEnv 与 CI 私密变量约定保持一致，签名私钥仅从此环境变量读取。
const privateKeyEnv = "ALLBOT_UPDATE_ED25519_PRIVATE_KEY"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "genkey":
		if err := runGenKey(); err != nil {
			fail(err)
		}
	case "sign":
		if err := runSign(os.Args[2:]); err != nil {
			fail(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

// runGenKey 生成密钥对并打印 base64 编码结果，私钥绝不写入磁盘以降低泄露风险。
func runGenKey() error {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("生成 Ed25519 密钥失败: %w", err)
	}
	fmt.Printf("private_key(base64): %s\n", base64.StdEncoding.EncodeToString(privateKey))
	fmt.Printf("public_key(base64):  %s\n", base64.StdEncoding.EncodeToString(publicKey))
	return nil
}

// runSign 读取私钥并对输入文件原始字节签名，输出 base64 签名文件。
func runSign(args []string) error {
	flagSet := flag.NewFlagSet("sign", flag.ContinueOnError)
	inPath := flagSet.String("in", "", "待签名文件路径（校验文件原文）")
	outPath := flagSet.String("out", "", "签名输出路径，默认 <in>.sig")
	if err := flagSet.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*inPath) == "" {
		return fmt.Errorf("必须通过 -in 指定待签名文件")
	}
	privateKey, err := loadPrivateKey()
	if err != nil {
		return err
	}
	payload, err := os.ReadFile(*inPath)
	if err != nil {
		return fmt.Errorf("读取待签名文件失败: %w", err)
	}
	if len(payload) == 0 {
		return fmt.Errorf("待签名文件为空")
	}
	signature := ed25519.Sign(privateKey, payload)
	encoded := base64.StdEncoding.EncodeToString(signature)

	target := strings.TrimSpace(*outPath)
	if target == "" {
		target = *inPath + ".sig"
	}
	if err := os.WriteFile(target, []byte(encoded), 0o644); err != nil {
		return fmt.Errorf("写入签名文件失败: %w", err)
	}
	fmt.Printf("已写入签名: %s\n", target)
	return nil
}

// loadPrivateKey 从环境变量解析 base64 私钥，长度必须与 crypto/ed25519 私钥一致。
func loadPrivateKey() (ed25519.PrivateKey, error) {
	encoded := strings.TrimSpace(os.Getenv(privateKeyEnv))
	if encoded == "" {
		return nil, fmt.Errorf("未配置签名私钥环境变量 %s", privateKeyEnv)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("签名私钥 base64 解析失败")
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("签名私钥长度无效，应为 %d 字节", ed25519.PrivateKeySize)
	}
	return ed25519.PrivateKey(decoded), nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "用法:")
	fmt.Fprintln(os.Stderr, "  updatesign genkey")
	fmt.Fprintln(os.Stderr, "  updatesign sign -in <文件> [-out <文件>]")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "错误:", err)
	os.Exit(1)
}
