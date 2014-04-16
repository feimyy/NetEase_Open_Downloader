package main

import (
	"bytes"
	"fmt"
	iconv "github.com/feimyy/iconv"
	"io"
	"io/ioutil"
	// "log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	filename_DisallowedChar string = "/\\:?*\"|" //windows系统文件名不被允许的字符
	retry_max_num                  = 10
	default_file_size       int64  = 1024 * 1024 * 1024 * 10 //10M
)

type ResourceInfo struct {
	Id       string
	DownUrL  string
	Suffix   string // don't contain the dot(.)
	Name     string
	Sequence int //该视频是第几集
}

func checkError(err error) {
	if err == io.EOF {
		return
	} else if err != nil {
		fmt.Println("Occured a error  :", err.Error())
		os.Exit(0)
	}

}

func getSourceCode(Url string) (sourceCode string) {

	rep, err := http.Get(Url)
	checkError(err)

	b, err := ioutil.ReadAll(rep.Body)
	checkError(err)

	sourceCode = fmt.Sprintf("%s", b)
	defer rep.Body.Close()

	return
}
func getPage() string {

	return "http://v.163.com/special/opencourse/americaprogram.html"
}

func getResourceDownloadList(PageSourceCode string) *[]ResourceInfo {

	//通过正则粗略匹配所有视频下载Url
	down_exp := "<a\\sclass=\"downbtn\"\\shref='[^<]*?target[^<]*?</a>"
	list := getDownloadList(down_exp, PageSourceCode)
	//fmt.Printf("list :%v\n\n", list)

	//根据粗略匹配到Url中的获取精确Url
	filted_list := filterDownloadList(list)
	//fmt.Printf("%v\n\n", filted_list)

	//获取页面的字符集
	charser_exp := "<meta http-equiv=[^>]*?\">"
	charset := getPageCharset(charser_exp, PageSourceCode)
	fmt.Printf("charset :%s\n", charset)

	//获取Url视频资源对应的名称
	ResourceList := make([]ResourceInfo, len(filted_list))
	for i, v := range filted_list {
		//特别注意提取名称的正则表达式
		name_exp := fmt.Sprintf("http://v\\.163\\.com.*?%s\\.[a-zA-Z0-9]{4}\">[^<]+</a>", v.Id)
		name := getResourceNameById(name_exp, PageSourceCode)

		converter, err := iconv.Open("utf-8", charset)
		if err != nil {
			fmt.Printf("iconv.Open() error :%s\n", err)
			panic(err)
		}

		name_reader := bytes.NewReader(name)
		converted_reader := iconv.NewReader(converter, name_reader, len(name))

		converted_name_bytes, _ := ioutil.ReadAll(converted_reader)
		buffer := bytes.NewBuffer(converted_name_bytes)
		v.Name = buffer.String() //保存转化后的名称
		//fmt.Printf("Id :%s ,Name :%s\n\n", v.Id, v.Name)

		ResourceList[i] = v

	}

	//设置视频集数
	for i, v := range ResourceList {
		v.Sequence = i + 1 //默认集数
		ResourceList[i] = getResourceSequence(PageSourceCode, v)
		fmt.Printf("[第%d集]%s.%s\n", v.Sequence, v.Name, v.Suffix)
	}

	return &ResourceList
}

//去掉重复的条目
func trimReduplicate(list []string) []string {
	trimmed_list := make([]string, 0)
	for i, v := range list {
		IsFound := false
		for j := i + 1; j < len(list); j++ {
			if strings.EqualFold(list[i], list[j]) { //在该字符后存在与该字符相同的字符
				IsFound = true
			}
		}
		if !IsFound {
			trimmed_list = append(trimmed_list, v)
		}
	}

	return trimmed_list
}

//通过正则获取包含了视频下载链接的内容段
func getDownloadList(exp string, PageSourceCode string) []string {
	reg, err := regexp.Compile(exp)
	checkError(err)
	list := reg.FindAllString(PageSourceCode, -1)
	trimmed_list := trimReduplicate(list)
	return trimmed_list
}

/*
   从正则表达式粗略匹配的包含了下载的内容段中，
   精确匹配出视频Url,后缀,Id
*/
func filterDownloadList(DownloadList []string) []ResourceInfo {
	down_exp := "href='[^']*?'"
	id_exp := "id='[^']*?'"
	down_reg, _ := regexp.Compile(down_exp)
	id_reg, _ := regexp.Compile(id_exp)

	ResourceList := make([]ResourceInfo, len(DownloadList))
	for i, v := range DownloadList {
		down_content := down_reg.FindString(v)
		id_content := id_reg.FindString(v)

		//fmt.Printf("down_content :%s ,id_content:%s\n", down_content, id_content)
		down_content_splited := strings.Split(down_content, "'")
		id_content_splited := strings.Split(id_content, "'")

		//fmt.Printf("len :%d ,down_content_splited :%v\n", len(down_content_splited), down_content_splited)
		//fmt.Printf("len : %d ,id_content_splited :%v\n", len(id_content_splited), id_content_splited)

		ResourceList[i].DownUrL = down_content_splited[len(down_content_splited)-2]
		ResourceList[i].Id = id_content_splited[len(id_content_splited)-2]

		//从下载Url中提取后缀名
		DownUrl_splited := strings.Split(ResourceList[i].DownUrL, ".")
		ResourceList[i].Suffix = DownUrl_splited[len(DownUrl_splited)-1]
	}

	return ResourceList
}

//获取原页面的字符编码
func getPageCharset(exp string, PageSourceCode string) string {

	charset := "gb2312" // default charset
	reg, _ := regexp.Compile(exp)
	meta := reg.FindString(PageSourceCode)
	if !strings.Contains(meta, "charset=") {
		return charset
	}

	contents := strings.Split(meta, "charset=")
	charset_content := contents[1]

	start := 0
	end := len(charset_content)
	for i := 0; i < len(charset_content); i++ {
		if charset_content[i] == '"' {
			end = i
			break
		}
	}

	charset = charset_content[start:end]
	return charset
}

/*
   通过视频下载Url的Id获取匹配视频的名称

   注意:返回的是原页面字符集编码的名称的[]byte数据
*/
func getResourceNameById(exp string, PageSourceCode string) []byte {

	reg, _ := regexp.Compile(exp)
	content := reg.Find([]byte(PageSourceCode))
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '>' {
			start = i + 1 //不包含'>'本身
			break
		}
	}

	end := len(content)
	for i := (len(content) - 1); i > 0; i-- {
		if content[i] == '<' {
			end = i
			break
		}
	}

	name := content[start:end]
	return name
}

//设置视频所属的集数
func getResourceSequence(PageSourceCode string, item ResourceInfo) ResourceInfo {

	//尝试使用正则从页面源码中找到视频所属的集数

	exp := fmt.Sprintf("_movie[\\d{1,3}] = {id:'%s'};", item.Id) //特定正则

	reg, err := regexp.Compile(exp)
	if err == nil {
		content := reg.FindString(PageSourceCode)

		//从内容中提取出[]中的数字
		start := strings.Index(content, "[") + 1
		end := strings.Index(content, "]")
		if start < 0 {
			start = 0
		}
		if end < 0 {
			end = len(content)
		}

		seq, err := strconv.Atoi(content[start:end])
		if err == nil {
			item.Sequence = seq + 1 //网页中数字是序号，集数比序号大一
		}

	}
	return item
}

//替换文件名中不被windows接受的文件名字符串
func filtLegalString(old string) string {

	for i := 0; i < len(filename_DisallowedChar); i++ {
		if strings.Contains(old, filename_DisallowedChar[i:i+1]) {
			old = strings.Replace(old, filename_DisallowedChar[i:i+1], "——", -1)
		}
	}
	return old
}

/*
   Desc:
       每一个Ventilator负责一个是视频的下载任务

   Params:
      chan : int返回消耗的时间,并且作为结束的标志

*/
func Ventilator(item ResourceInfo, RoutineCount int, elapsee_senconds chan int) {

	//创建文件
	file_name := fmt.Sprintf("[第%d集]%s.%s", item.Sequence, item.Name, item.Suffix)
	file_name = filtLegalString(file_name)
	file, err := os.Create(file_name)
	if err != nil {
		fmt.Printf("os.Create() ,file_name :%s ,error :%s\n", file_name, err)
	}
	defer file.Close()

	fmt.Printf("开始下载视频:%s\n", item.Name)

	//获得目标文件大小
	req, err := http.NewRequest("GET", item.DownUrL, nil)
	if err != nil {
		fmt.Printf("不能创建Http Request :Url:%s,error: %s\n", item.DownUrL, err)
		elapsee_senconds <- -1
		return
	}

	var rep *http.Response = nil
	client := &http.Client{}
	for i := retry_max_num; i > 0; i-- {
		rep, err = client.Do(req)
		if err != nil {
			fmt.Printf("不能连接到指定的Url :%s,error :%s\n\tRetry it....\n", item.DownUrL)
		} else {
			break
		}
	}
	if rep == nil { //获取Response失败
		elapsee_senconds <- -1 //该任务失败，该任务需要重试
		return
	}
	var ContentLength int64 = 0
	ContentLength = rep.ContentLength
	if ContentLength == -1 {
		fmt.Printf("目标文件大小未知,只能使用单线程下载\n")
		RoutineCount = 1
	} else {
		fmt.Printf("文件名:%s,\t大小:%dMb \n\t\tUrl %s\n", file_name, (ContentLength / (1024 * 1024)), item.DownUrL)
		if RoutineCount > 1 { //即使服务端支持多线程，只有routineCount大于1才改变大小
			err = file.Truncate(ContentLength) //使用大小填充指定文件
			if err != nil {
				fmt.Printf("不能改变指定文件大小 :%s\n", err)
				elapsee_senconds <- -1
				return
			}
		}
	}

	//测试目标是否支持Http Header的Range属性
	req.Header.Add("Range", "bytes=0-1")
	for i := retry_max_num; i > 0; i-- {
		rep, err = client.Do(req)
		if err != nil {
			fmt.Printf("不能连接到指定的Url :%s,error :%s\n\tRetry it....\n", item.DownUrL)
		} else {
			break
		}
	}
	if rep.StatusCode != 206 {
		RoutineCount = 1
		fmt.Printf("目标主机不支持多线程下载!,将使用单线程下载\n")
	} else {
		fmt.Printf("目标主机支持多线程下载\n")
	}

	//test
	channel := make(chan int)
	if ContentLength == 0 {
		go Worker(file, item.DownUrL, 0, 0, channel)
	} else {
		go Worker(file, item.DownUrL, 0, ContentLength-1, channel)
	}
	<-channel

	//end test
	elapsee_senconds <- item.Sequence
}

func Worker(file *os.File, Url string, startOffset, endOffset int64, channnel chan int) {
	Stat, _ := file.Stat()
	size := Stat.Size()
	if size > 0 { //多线程

		req, err := http.NewRequest("GET", Url, nil)
		if err != nil {
			fmt.Printf("不能创建Http Request :Url:%s,error: %s\n", Url, err)
			channnel <- -1
			return
		}
		RangeValue := fmt.Sprintf("bytes=%d-%d", startOffset, endOffset)
		req.Header.Add("Range", RangeValue)

		client := &http.Client{}
		var rep *http.Response = nil
		for i := retry_max_num; i > 0; i-- {
			rep, err = client.Do(req)
			if err != nil {
				fmt.Printf("不能连接到指定的Url :%s,error :%s\n\tRetry it....\n", Url, err)
			} else {
				break
			}
		}
		if rep == nil {
			channnel <- -1 //该任务失败，该任务需要重试
			return
		}

	} else { //单线程

		fmt.Println("开始单线程下载......")
		var rep *http.Response = nil
		var err error
		for i := retry_max_num; i > 0; i-- {
			rep, err = http.Get(Url)
			if err != nil {
				fmt.Printf("不能连接到指定的Url :%s,error :%s\n\tRetry it....\n", Url)
			} else {
				break
			}
		}
		if rep == nil {
			channnel <- -1 //该任务失败，该任务需要重试
			return
		}

		if (endOffset - startOffset) != 0 { //目标文件大小已知
			file.Truncate((endOffset - startOffset + 1)) //改变文件大小
			var i int
			for i = retry_max_num; i > 0; i-- {
				written, err := io.Copy(file, rep.Body)
				if err == nil && written == (endOffset-startOffset+1) {
					break
				} else {
					fmt.Printf("io.Copy() ,error:%s\n", err)
					panic(err)
				}
				file.Truncate(0) //清空文件
			}

			if i < 0 {
				channnel <- -1
			} else {
				channnel <- 1
			}
			defer rep.Body.Close()
		} else if endOffset == startOffset { //目标文件大小未知,只能使用边写边增加文件大小
			/*
			 TODO : 更好的方法实现下载未知大小的文件
			*/
			//这个方法针对大文件太耗费内存了
			/*b, err1 := ioutil.ReadAll(rep.Body)
			defer rep.Body.Close()
			if err1 != nil {
				channnel <- -1
			}
			file.Write(b) //写文件*/
			var i int64 = 0
			for ; !rep.Close; i++ {
				file.Truncate((i + 1) * default_file_size)
				n, err2 := io.CopyN(file, rep.Body, default_file_size)
				if err2 != io.EOF {
					file.Seek(default_file_size, 1) //移动偏移
				} else if err2 == io.EOF {
					file.Truncate(i*default_file_size + n)
					break
				}
			}
			defer rep.Body.Close()
			channnel <- 0
		}
	}
}
func main() {

	/*	Url := getPage()
		sourceCode := getSourceCode(Url)
		list := getResourceDownloadList(sourceCode)

		channels := make([]chan int, len(*list))
		for i, v := range *list {
			channels[i] = make(chan int)
			fmt.Printf("第%d集\n\tUrl :%s\n\tName :%s.%s\n", v.Sequence, v.DownUrL, v.Name, v.Suffix)
			go Ventilator(v, 10, channels[i])
		}

		for i, v := range channels {

			<-v
			fmt.Printf("第%d下载者退出\n", i)
		}*/

	Test := new(ResourceInfo)
	Test.Name = "haozip_v4.3_jt"
	Test.Suffix = "exe"
	Test.Sequence = 0
	Test.DownUrL = "http://download.2345.com/haozip/haozip_v4.3_jt.multi.exe"
	test_channnel := make(chan int)
	go Ventilator(*Test, 1, test_channnel)
	re := <-test_channnel
	if re == -1 {
		fmt.Printf("the test is failed\n")
	} else {
		fmt.Printf("the test download is successeful")
	}
}
