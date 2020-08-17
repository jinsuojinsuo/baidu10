package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/360EntSecGroup-Skylar/excelize/v2"
	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const Sheet1 = "Sheet1"
const baiduUrl = "https://www.baidu.com"

var _page = 3
var wrod = ""
var openChrome bool
var outFileName string

func init() {
	flag.IntVar(&_page, "p", _page, "要获取的总页数,默认: 3")
	flag.StringVar(&wrod, "w", wrod, "要搜索的关键词,不能为空")
	flag.BoolVar(&openChrome, "s", openChrome, "是否隐藏浏览器 默认false不隐藏")
	flag.StringVar(&outFileName, "o", outFileName, "保存文件名,默认当前时间20200701T010101")
}

func main() {
	flag.Parse()
	if outFileName == "" {
		outFileName = wrod + time.Now().Format("20060102T150405") + ".xlsx"
	}
	if strings.ToLower(path.Ext(outFileName)) != ".xlsx" {
		outFileName = outFileName + ".xlsx"
	}

	//获取不带后缀的文件名
	fileName := strings.TrimSuffix(outFileName, ".xlsx")

	if strings.TrimSpace(wrod) == "" {
		log.Fatalln("搜索内容不能为容")
	}
	colIndex := 0 //列
	rowIndex := 1 //行
	excelFile := excelize.NewFile()
	index := excelFile.NewSheet(Sheet1) // 创建一个工作表
	excelFile.SetCellValue(Sheet1, ExcelPos(colIndex+0, rowIndex), "页码")
	excelFile.SetCellValue(Sheet1, ExcelPos(colIndex+1, rowIndex), "标题")
	excelFile.SetCellValue(Sheet1, ExcelPos(colIndex+2, rowIndex), "url")
	excelFile.SetCellValue(Sheet1, ExcelPos(colIndex+3, rowIndex), "描述")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.Flag("headless", openChrome), //false 打开
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	// create chrome instance
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	//设置chrome打开到关闭的总时长
	//ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()

	// navigate to a page, wait for an element, click

	pageCode := 1 //页码
	log.Printf("第%d页\n", pageCode)

	var ScreenshotBuf []byte //图片缓冲区

	var contentLeft string
	var pageHTML string //页码列表
	if err := chromedp.Run(ctx,
		chromedp.Navigate(baiduUrl),                                                               //访问打开页面
		chromedp.SendKeys("#kw", wrod, chromedp.NodeVisible, chromedp.ByID),                       //搜索框内输入zhangguanzhang
		chromedp.Click(".s_btn", chromedp.NodeVisible, chromedp.ByQuery),                          // 点击搜索图标
		chromedp.OuterHTML("#content_left", &contentLeft, chromedp.NodeVisible, chromedp.ByQuery), //获取内容
		chromedp.OuterHTML("#page", &pageHTML, chromedp.NodeVisible, chromedp.ByQuery),
		chromedp.ActionFunc(cbMaxScreen),
		chromedp.Screenshot("body", &ScreenshotBuf, chromedp.NodeVisible, chromedp.ByQuery),
	); err != nil {
		log.Fatal("执行失败", err)
	}

	//保存截图
	saveScreen(fileName+"_"+strconv.Itoa(pageCode), ScreenshotBuf)

	//fmt.Println(contentLeft)
	GetOnepage(contentLeft, pageCode, &colIndex, &rowIndex, excelFile) //抓取内容写入excel

	_page = _page - 1
	for i := 0; i < _page; i++ {
		pageCode = i + 2
		log.Printf("第%d页\n", pageCode)
		jqdom, err := goquery.NewDocumentFromReader(strings.NewReader(pageHTML))
		if err != nil {
			log.Fatalln("创建jqdom失败")
		}
		contentLeft = ""
		pageHTML = "" //页码列表
		if nextUrl, ok := jqdom.Find(".page-inner > .n").Last().Attr("href"); ok == true {
			chromedp.Run(ctx,
				chromedp.Navigate(baiduUrl+nextUrl),
				chromedp.OuterHTML("#content_left", &contentLeft, chromedp.NodeVisible, chromedp.ByQuery), //获取内容
				chromedp.OuterHTML("#page", &pageHTML, chromedp.NodeVisible, chromedp.ByQuery),
				chromedp.Screenshot("body", &ScreenshotBuf, chromedp.NodeVisible, chromedp.ByQuery),
			)
			saveScreen(fileName+"_"+strconv.Itoa(pageCode), ScreenshotBuf)     //保存截图
			GetOnepage(contentLeft, pageCode, &colIndex, &rowIndex, excelFile) //抓取内容写入excel
		}
	}

	// 设置工作簿的默认工作表
	excelFile.SetActiveSheet(index)

	// 根据指定路径保存文件
	if err := excelFile.SaveAs(outFileName); err != nil {
		fmt.Println(err)
	}
	log.Println("完成")
}

//抓取内容写入excel
func GetOnepage(contentLeft string, pageCode int, colIndex, rowIndex *int, f *excelize.File) {
	jqdom, err := goquery.NewDocumentFromReader(strings.NewReader(contentLeft))
	if err != nil {
		log.Fatalln("创建jqdom失败")
	}

	jqdom.Find("#content_left > .c-container").Each(func(i int, s *goquery.Selection) {
		u, _ := s.Find(".t a,header a").Attr("href")
		title := strings.TrimSpace(s.Find(".t,.c-title").Text())
		content := strings.TrimSpace(s.Find(".c-abstract").Text())

		if u2, err := url.Parse(u); err != nil {
			fmt.Println("url解析失败", title, u)
		} else if u2.Host == "" {
			u = baiduUrl + u2.String()
		}

		if u != "" {
			code, _, locationUrl, err := HttpGet(u, map[string]string{})
			if err != nil {
				log.Println("请求失败", title, code, "url:", u, err)
			}
			if locationUrl != "" {
				u = locationUrl
			}
		}
		*rowIndex++
		if err := f.SetCellValue(Sheet1, ExcelPos(*colIndex+0, *rowIndex), pageCode); err != nil {
			log.Println("写入失败", err)
		}
		if err := f.SetCellValue(Sheet1, ExcelPos(*colIndex+1, *rowIndex), title); err != nil {
			log.Println("写入失败", err)
		}
		if err := f.SetCellValue(Sheet1, ExcelPos(*colIndex+2, *rowIndex), u); err != nil {
			log.Println("写入失败", err)
		}
		if err := f.SetCellValue(Sheet1, ExcelPos(*colIndex+3, *rowIndex), content); err != nil {
			log.Println("写入失败", err)
		}
	})
}

//httpGet 请求 响应内容过大不建议使用此方法
func HttpGet(url string, header map[string]string) (code int, body []byte, locationUrl string, err error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, "", err
	}
	for k, v := range header {
		req.Header.Add(k, v) //请求类型
	}
	client := &http.Client{
		Timeout: 10 * time.Second, //设置超时时间
		CheckRedirect: func(req *http.Request, via []*http.Request) error { //遇到302禁止自动重定向
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, "", err
	}
	if resp != nil {
		defer resp.Body.Close()
	} else {
		return 0, nil, "", errors.New("http.Request not is nil")
	}
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, "", err
	}
	locationUrl = resp.Header.Get("Location")
	return resp.StatusCode, body, locationUrl, nil
}

//数字转字母, 从0开始
//0=A,1=B,26=AA,27=AB
func AZ26(i int) string {
	var Letters = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z"}
	r := Letters[i%26]
	i = i / 26
	for i > 0 {
		i = i - 1
		r = Letters[i%26] + r
		i = i / 26
	}
	return r
}

//生成excel坐标(列,行)
//列是从0开始 行是从1开始
func ExcelPos(col int, row int) string {
	return strings.Join([]string{AZ26(col), strconv.Itoa(row)}, "")
}

//回调函数 强制视口仿真 (模拟一个宽=width 高=height 的大屏幕)
func cbMaxScreen(ctx context.Context) error {

	//返回与页面布局相关的度量，例如视口边界/比例。
	_, _, contentSize, err := page.GetLayoutMetrics().Do(ctx)
	if err != nil {
		return err
	}

	//获取页面宽高,包含非可视区域
	width, height := int64(math.Ceil(contentSize.Width)), int64(math.Ceil(contentSize.Height))

	// force viewport emulation 强制视口仿真 (模拟一个宽=width 高=height 的大屏幕)
	if err := emulation.SetDeviceMetricsOverride(width, height, 1, false).WithScreenOrientation(&emulation.ScreenOrientation{
		Type:  emulation.OrientationTypePortraitPrimary,
		Angle: 0,
	}).Do(ctx); err != nil {
		return err
	}
	return nil
}

//保存截图
func saveScreen(fileName string, buf []byte) {
	if err := ioutil.WriteFile(fileName+".png", buf, 0644); err != nil {
		log.Fatal(err)
	}
}
