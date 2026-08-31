package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	inputDir  *string
	outputDir *string
)

func main() {
	loadPath()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Printf("创建输出目录失败: %v\n", err)
		return
	}

	files, err := os.ReadDir(*inputDir)
	if err != nil {
		fmt.Printf("读取输入目录失败: %v\n", err)
		return
	}

	for _, f := range files {
		if f.IsDir() || (!strings.HasSuffix(strings.ToLower(f.Name()), ".wmf") && !strings.HasSuffix(strings.ToLower(f.Name()), ".emf")) {
			continue
		}
		inputPath := filepath.Join(*inputDir, f.Name())
		data, err := os.ReadFile(inputPath)
		if err != nil {
			fmt.Printf("读取文件 %s 失败: %v\n", f.Name(), err)
			continue
		}

		ext := strings.ToLower(filepath.Ext(f.Name()))
		baseName := strings.TrimSuffix(f.Name(), ext)

		if ext == ".wmf" {
			svg, err := convertWmfToSvg(data)
			if err != nil {
				fmt.Printf("转换文件 %s 失败: %v\n", f.Name(), err)
				continue
			}
			outputPath := filepath.Join(*outputDir, baseName+".svg")
			if err := os.WriteFile(outputPath, []byte(svg), 0644); err != nil {
				fmt.Printf("写入文件 %s 失败: %v\n", outputPath, err)
				continue
			}
			fmt.Printf("成功转换: %s -> %s.svg\n", f.Name(), baseName)
		}
	}
}

func loadPath() {
	// 定义命令行flag参数
	inputDir = flag.String("input", "", "输入目录路径，必填")
	outputDir = flag.String("output", "", "输出目录路径，必填")

	// 解析命令行
	flag.Parse()

	// 校验必填
	if *inputDir == "" {
		// 打印帮助
		flag.Usage()
		os.Exit(1)
	}
	if *outputDir == "" {
		*outputDir = "./"
	}
}
