# baidu10

抓取百度搜索结果前10页的内容放入excel文件

# 安装
go get github.com/jinsuojinsuo/baidu10

# 选项
    PS D:\> baidu10.exe --help
    Usage of C:\Users\yunwe\go\bin\baidu10.exe:
      -o string
            保存文件名,默认当前时间20200701T010101
      -p int
            要获取的总页数,默认: 3 (default 3)
      -s    是否隐藏浏览器 默认false不隐藏
      -w string
            要搜索的关键词,不能为空`


# 使用
baidu10.exe -w 搜什么 -s

