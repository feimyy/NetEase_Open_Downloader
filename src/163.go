/*
   Author : feimyy <feimyy@hotmail.com>
   CopyRight : Apache License 2.0
*/
package main

import (
	"bytes"
	"fmt"
	iconv "github.com/feimyy/iconv"
	"io"
	"io/ioutil"
	// "log"
	"flag"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	filename_DisallowedChar string = "/\\:?*\"|" //windows系统文件名不被允许的字符
	retry_max_num                  = 3
	default_file_size       int64  = 1024 * 1024 * 1024 * 10 //10M
	default_routine_num            = 10
)

type ResourceInfo struct {
	Id       string
	DownUrl  string
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

		ResourceList[i].DownUrl = down_content_splited[len(down_content_splited)-2]
		ResourceList[i].Id = id_content_splited[len(id_content_splited)-2]

		//从下载Url中提取后缀名
		DownUrl_splited := strings.Split(ResourceList[i].DownUrl, ".")
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

func GetTargetResourceContentLength(Url string) int64 {
	req, err := http.NewRequest("GET", Url, nil)
	if err != nil {
		fmt.Printf("不能创建Http Request :Url:%s,error: %s\n", Url, err)
		return -1
	}

	var rep *http.Response = nil
	client := &http.Client{}
	for i := retry_max_num; i > 0; i-- {
		rep, err = client.Do(req)
		if err != nil {
			fmt.Printf("不能连接到指定的Url :%s,error :%s\n\tRetry it....\n", Url)
		} else {
			break
		}
	}
	if rep == nil { //获取Response失败
		return -1
	}
	if rep.ContentLength != -1 {
		return rep.ContentLength
	} else {
		return 0
	}
}

func IsSupportMultiThread(Url string) int {
	req, err := http.NewRequest("GET", Url, nil)
	if err != nil {
		fmt.Printf("不能创建Http Request :Url:%s,error: %s\n", Url, err)
		return -1
	}
	req.Header.Add("Range", "bytes=0-1")

	var rep *http.Response = nil
	client := &http.Client{}
	for i := retry_max_num; i > 0; i-- {
		rep, err = client.Do(req)
		if err != nil {
			fmt.Printf("不能连接到指定的Url :%s,error :%s\n\tRetry it....\n", Url)
		} else {
			break
		}
	}
	if rep == nil { //获取Response失败
		return -1 //发生了网络连接错误
	}
	if rep.StatusCode == 206 {
		return 1 //支持
	} else {
		return 0 //不支持
	}

}
func Ventilator(item ResourceInfo, RoutineCount int, elapsee_senconds chan int64) {

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
	ContentLength := GetTargetResourceContentLength(item.DownUrl)
	if ContentLength == 0 {
		fmt.Printf("目标文件大小未知,只能使用单线程下载\n")
		RoutineCount = 1
	} else if ContentLength > 0 {
		fmt.Printf("文件名:%s\n\t大小: %d Mb\n\tBytes : %d bytes\n\tUrl: %s\n", file_name, (ContentLength / (1024 * 1024)), ContentLength, item.DownUrl)
		if RoutineCount > 1 { //即使服务端支持多线程，只有routineCount大于1才改变大小
			err = file.Truncate(ContentLength) //使用大小填充指定文件
			if err != nil {
				fmt.Printf("不能改变指定文件大小 :%s\n", err)
				elapsee_senconds <- -1
				return
			}
		}
	} else if ContentLength == -1 {
		elapsee_senconds <- -1
		return
	}

	//测试目标是否支持Http Header的Range属性
	IsSupport := IsSupportMultiThread(item.DownUrl)
	if IsSupport == 0 {
		RoutineCount = 1
		fmt.Printf("目标主机不支持多线程下载!,将使用单线程下载\n")
	} else if IsSupport == 1 {
		fmt.Printf("目标主机支持多线程下载\n")
	} else if IsSupport == -1 {
		elapsee_senconds <- -1
		return
	}

	if RoutineCount == 1 { //单线程
		channel := make(chan int64)
		startTime := time.Now().UnixNano()
		endTime := time.Now().UnixNano()
		if ContentLength != 0 { //目标文件大小已知
			var i int = 0
			for i = retry_max_num; i > 0; i-- {
				go Worker(file, item.DownUrl, 0, ContentLength-1, channel)
				r := <-channel
				if r != -1 { //routine成功下载文件
					endTime = time.Now().UnixNano()
					fmt.Printf("文件 %s 的下载Routine下载成功，消耗时间:%d s\n", file_name, (endTime-startTime)/int64(time.Second))
					break
				}
			}
			if i < 0 {
				elapsee_senconds <- -1
			} else {
				elapsee_senconds <- ((endTime - startTime) / int64(time.Second))
			}
		} else if ContentLength == 0 { //目标文件大小未知

			//创建goroutine下载文件
			var i int = 0
			for i = retry_max_num; i > 0; i-- {
				go Worker(file, item.DownUrl, 0, 0, channel)
				r := <-channel
				if r != -1 { //routine成功下载文件
					endTime = time.Now().UnixNano()
					fmt.Printf("文件 %s 的下载Routine下载成功，消耗时间:%d s\n", file_name, (endTime-startTime)/int64(time.Second))
					break
				}
			}
			if i < 0 {
				elapsee_senconds <- -1
			} else {
				elapsee_senconds <- ((endTime - startTime) / int64(time.Second))
			}
		}
	} else { //多线程下载
		channels := make([]chan int64, 0)

		block := ContentLength / int64(RoutineCount)
		if ContentLength%int64(RoutineCount) != 0 {
			block += 1
		}

		//创建并运行Routines
		files := make([]*os.File, RoutineCount)
		for i := 0; i < RoutineCount; i++ {

			/*
			   为每一个Routine创建一个只写的文件操作Handle
			   如果所有的Routine都是用同一个文件操作句柄的话
			   那个每个file.Seek时可能发生另一个routine改变了文件指针，导致当前Routine将数据写入错误的位置
			*/
			fmt.Printf("文件 %s :开始第 %d 个下载Routine\n", file_name, i)
			files[i], err = os.OpenFile(file_name, os.O_WRONLY, 0666)
			defer files[i].Close()
			if err != nil {
				fmt.Printf("不能打开指定文件 :%s\n", err)
				os.Exit(0)
			}

			v := make(chan int64)
			channels = append(channels, v)

			start := block * int64(i)
			end := block*int64(i+1) - 1
			if end >= ContentLength {
				end = ContentLength - 1
			}

			go Worker(files[i], item.DownUrl, start, end, v)
		}

		//等待Routines返回
		var costSenconds int64 = 0
		for i, v := range channels {
			fmt.Printf("文件 %s :开始等待第 %d 个下载Routine退出\n", file_name, i)
			r := <-v
			if r != -1 { //routine成功下载文件
				fmt.Printf("文件 %s 的 第 %d 个下载Routine下载成功，消耗时间:%d s\n", file_name, i, r)
				costSenconds += r
			} else { //该routine失败,重试之
				fmt.Printf("文件 %s 的 第 %d 个下载Routine下载失败,重试...\n", file_name, i)

				start := block * int64(i)
				end := block*int64(i+1) - 1
				if end >= ContentLength {
					end = ContentLength - 1
				}

				var j int = 0
				for j = retry_max_num; j > 0; j-- {
					go Worker(file, item.DownUrl, start, end, v)
					new_r := <-v
					if new_r != -1 {
						fmt.Printf("文件 %s 的 第 %d 个下载Routine重新下载成功，消耗时间:%d s\n", file_name, i)
						break
					}
				}

				if j < 0 { //下载分段失败
					errInfo := fmt.Sprintf("文件: %s ,Range :%d-%d :下载失败\n", file_name, start, end)
					logFile, err := os.OpenFile(item.Name+".err.log", os.O_APPEND|os.O_WRONLY, 0666)
					defer logFile.Close()
					if err != nil {
						fmt.Printf("创建Log文件失败: %s\n", err)
					} else {
						logFile.WriteString(errInfo)
					}

				}

			}
		}
		elapsee_senconds <- costSenconds
	}
}

func Worker(file *os.File, Url string, startOffset, endOffset int64, channel chan int64) {
	Stat, _ := file.Stat()
	size := Stat.Size()
	if size > 0 && endOffset > startOffset { //多线程

		startTime := time.Now().UnixNano()
		req, err := http.NewRequest("GET", Url, nil)
		if err != nil {
			fmt.Printf("不能创建Http Request :Url:%s,error: %s\n", Url, err)
			channel <- -1
			return
		}
		RangeValue := fmt.Sprintf("bytes=%d-%d", startOffset, endOffset)
		req.Header.Add("Range", RangeValue)
		req.Header.Add("Accept-Encoding", "identity")
		client := &http.Client{}
		var rep *http.Response = nil
		for i := retry_max_num; i > 0; i-- {
			rep, err = client.Do(req)
			if err != nil {
				fmt.Printf("不能连接到指定的Url :%s\n,\terror :%s\n\tRetry it....\n", Url, err)
			} else {
				break
			}
		}
		if rep == nil {
			channel <- -1 //该任务失败，该任务需要重试
			return
		}

		var i int = 0
		file.Seek(startOffset, 0)
		var needSize int64 = (endOffset - startOffset + 1)
		for i = retry_max_num; i > 0; i-- {
			written, err := io.CopyN(file, rep.Body, needSize)
			if err == nil && written == needSize {
				fmt.Printf("File :%s ,Range : %d-%d ,needSize :%d ,written :%d\n", Stat.Name(), startOffset, endOffset, needSize, written)
				break
			} else if err != io.EOF {
				fmt.Printf("io.CopyN() error: %s Range : %d - %d ,error:%s\n", Stat.Name(), startOffset, endOffset, err)
				channel <- -1
				return
			} else if err == io.EOF {
				fmt.Printf("File :%s ,Range : %d-%d ,needSize :%d ,written :%d\n", Stat.Name(), startOffset, endOffset, written)
				break
			}
		}

		defer rep.Body.Close()
		if i < 0 {
			channel <- -1
			return
		} else {

			endTime := time.Now().UnixNano()
			fmt.Printf("Download the Range  is successful : File '%s',  Range : %s , Elapsed_time : %ds\n", file.Name(), RangeValue, (endTime-startTime)/int64(time.Second))
			channel <- (endTime - startTime) / int64(time.Second)
			return
		}

	} else { //单线程
		start := time.Now().UnixNano()
		end := time.Now().UnixNano()
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
			channel <- -1 //该任务失败，该任务需要重试
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
			defer rep.Body.Close()
			if i < 0 {
				channel <- -1
				return
			} else {
				end = time.Now().UnixNano()
				channel <- ((end - start) / int64(time.Second))
				return
			}

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
			end = time.Now().UnixNano()
			channel <- ((end - start) / int64(time.Second))
			return
		}
	}
}

func Usage() {
	fmt.Printf("%s\t--url\t[-rnum]\n", strings.Join(os.Args[0:1], ""))
	fmt.Printf("\t--url :\n")
	fmt.Printf("\t\tThe url of opencourse\n")
	fmt.Printf("\t--rnum :\n")
	fmt.Printf("\t\tThe number of threads\n\t\tThe param is optional,and the default value is 10\n")
	fmt.Printf("\nFor example : \n\t%s --url http://v.163.com/special/justice/ --rnum 20\n", strings.Join(os.Args[0:1], ""))
}
func main() {

	if len(os.Args) < 2 {
		Usage()
		return
	}

	var Url string
	var RoutineNum int
	flag.StringVar(&Url, "url", "", "the url of opencourse")
	flag.IntVar(&RoutineNum, "rnum", default_routine_num, "the number of threads")
	flag.Parse()

	if len(Url) == 0 {
		Usage()
		return
	}
	sourceCode := getSourceCode(Url)
	list := getResourceDownloadList(sourceCode)

	channels := make([]chan int64, 0)
	for i, v := range *list {
		channel := make(chan int64)
		channels = append(channels, channel)
		for j := retry_max_num; j > 0; j-- {
			go Ventilator(v, RoutineNum, channel)
			r := <-channel
			if r != -1 {
				fmt.Printf("文件 ‘%s’ 下载完成!!！ 消耗时间:%ss\n", (*list)[i].Name+"."+(*list)[i].Suffix, r)
				break
			}
		}

	}

	//test code
	/*Test := new(ResourceInfo)
	Test.Name = "1"
	Test.Suffix = "iso"
	Test.Sequence = 0
	Test.DownUrl = Url
	test_channnel := make(chan int64)
	Ventilator(*Test, RoutineNum, test_channnel)
	re := <-test_channnel
	if re == -1 {
		fmt.Printf("the test is failed\n")
	} else {
		fmt.Printf("the test download is successeful")
	}*/
}
