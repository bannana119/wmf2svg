# wmf2svg
把wmf格式文件转换为svg格式文件，支持文件夹下批量转换，支持中文转换

```shell
go build .

./wmf2svg -input=input_folder -output=output_folder
```
## 使用方法
1. 下载并安装Go语言环境
2. 将wmf文件放入input_folder文件夹
3. 运行命令： 
```shell 
go build . && ./wmf2svg -input=input_folder -output=output_folder
````
4. 转换后的svg文件将保存在output_folder文件夹，output为空时文件输出 ```./```

## 交叉编译
1. 下载并安装Go语言环境
2. 运行命令：
```shell
# linux
GOOS=linux GOARCH=amd64 go build .
```

```shell
# windows
GOOS=windows GOARCH=amd64 go build .
```

```shell
# mac
GOOS=darwin GOARCH=amd64 go build .

```
3. 转换后的可执行文件将保存在当前目录



## 支持
如果觉得有用的话，可以支持一下，谢谢

|      |        |
| ---- | ------ |
| <img src="./image/wx.jpg" width="300"/> | <img src="./image/zfb.jpg" width="300"/> |
