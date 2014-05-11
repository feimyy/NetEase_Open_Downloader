/*
   Author : feimyy <feimyy@hotmail.com>
   CopyRight : Apache License 2.0
*/
package main

import (
	"bytes"
	"fmt"
	iconv "github.com/feimyy/iconv"
	logex "github.com/feimyy/log"
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
	filename_DisallowedChar string        = "/\\:?*\"|" //windows系统文件名不被允许的字符
	retry_max_num                         = 3
	default_file_size       int64         = 1024 * 1024 * 1024 * 10 //10M
	default_routine_num                   = 10
	is_verbose              bool          = false
	logfile_level           int           = Linfo
	logger                  *logex.Logger = nil
	stdoutLogger            *logex.Logger = logex.Std //default value
)

const (
	Ldebug = logex.Ldebug
	Linfo  = logex.Linfo
	Lerror = logex.Lerror
	Lwarn  = logex.Lwarn
)

var level_suffix = map[int]string{
	Ldebug: ".DEBUG.log",
	Linfo:  ".INFO.log",
	Lwarn:  ".WARN.log",
	Lerror: ".ERROR.log",
}

type ResourceInfo struct {
	Id       string
	DownUrl  string
	Suffix   string // don't contain the dot(.)
	Name     string
	Sequence int //该视频是第几集
}

type EpisodeList struct {
	needlist []int
}

func (e *EpisodeList) ParseValue(value string, episode_num int) bool {

	if e.IsNeedPartial() {

		if strings.Contains(value, ",") { //--episode 1,2,3
			list := strings.Split(value, ",")
			stdoutLogger.Debugf("multi episode :")
			for _, v := range list {
				n, err := strconv.Atoi(v)
				if err != nil {
					stdoutLogger.Debugf("strconv.Atoi() ,value : :%s,err :%s\n", value, err)
					return true
				} else {
					e.needlist = append(e.needlist, n)
				}

			}
			stdoutLogger.Debugf("\n")
			return false
		} else if strings.Contains(value, "-") { //--episode 1-10
			list := strings.Split(value, "-")
			if len(list) > 2 {
				return true
			}

			stdoutLogger.Debugf("episode range mode\n")
			//parse the start position
			n, err := strconv.Atoi(list[0])
			if err != nil {
				stdoutLogger.Debugf("strconv.Atoi() ,value : :%s,err :%s\n", value, err)
				return true
			} else {
				e.needlist = append(e.needlist, n)
			}

			//parse the end position
			if len(list[1]) == 0 { // --episode 1-
				for i := n + 1; i <= episode_num; i++ {
					e.needlist = append(e.needlist, i)
				}
				return false
			} else {
				n1, err := strconv.Atoi(list[1])
				if err != nil {
					return true
				} else {
					for i := n + 1; i <= n1; i++ {
						e.needlist = append(e.needlist, i)
					}

				}
				return false
			}
		} else {

			//--episode 1
			n, err := strconv.Atoi(value)
			if err == nil {
				e.needlist = append(e.needlist, n)
				stdoutLogger.Debugf("Single episode :%d,list :%v\n", n, e.needlist)
				return false
			} else {
				stdoutLogger.Debugf("strconv.Atoi() ,value : :%s,err :%s\n", value, err)
				return true
			}
		}
	} else {
		return true
	}
}

func (e EpisodeList) IsNeedEpisode(seq int) bool {
	if len(e.needlist) > 0 {
		for _, v := range e.needlist {
			if v == seq {
				return true
			}
		}
	}
	return false
}

//whether the episode flag be setted
func (e EpisodeList) IsNeedPartial() bool {
	for _, v := range os.Args[1:] {
		if strings.Contains(v, "episode") {
			return true
		}
	}

	return false
}
func checkError(err error) {
	if err == io.EOF {
		return
	}
	if err != nil {
		fmt.Printf("occured an error:%s\n", err)
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

	stdoutLogger.Debugf("list :%v\n\n", list)

	//根据粗略匹配到Url中的获取精确Url
	filted_list := filterDownloadList(list)

	stdoutLogger.Debugf("filted_list :%v\n\n", filted_list)

	//获取页面的字符集
	charser_exp := "<meta http-equiv=[^>]*?\">"
	charset := getPageCharset(charser_exp, PageSourceCode)

	stdoutLogger.Debugf("charset :%s\n", charset)

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

		stdoutLogger.Debugf("Id :%s ,Name :%s\n\n", v.Id, v.Name)

		if logger != nil {
			logger.Debugf("Id :%s,Name :%s\n\n", v.Id, v.Name)
		}
		ResourceList[i] = v

	}

	//设置视频集数
	for i, v := range ResourceList {
		v.Sequence = i + 1 //默认集数
		ResourceList[i] = getResourceSequence(PageSourceCode, v)
		stdoutLogger.Infof("[第%d集]%s.%s\n", v.Sequence, v.Name, v.Suffix)
		if logger != nil {
			logger.Infof("[第%d集]%s.%s\n", v.Sequence, v.Name, v.Suffix)
		}
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

		//output

		stdoutLogger.Debugf("down_content :%s ,id_content:%s\n", down_content, id_content)
		if logger != nil {
			logger.Debugf("down_content :%s ,id_content:%s\n", down_content, id_content)
		}
		//end output

		down_content_splited := strings.Split(down_content, "'")
		id_content_splited := strings.Split(id_content, "'")

		stdoutLogger.Debugf("len :%d ,down_content_splited :%v\n", len(down_content_splited), down_content_splited)
		stdoutLogger.Debugf("len : %d ,id_content_splited :%v\n", len(id_content_splited), id_content_splited)

		if logger != nil {
			logger.Debugf("len :%d ,down_content_splited :%v\n", len(down_content_splited), down_content_splited)
			logger.Debugf("len : %d ,id_content_splited :%v\n", len(id_content_splited), id_content_splited)
		}

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
		stdoutLogger.Errorf("不能创建Http Request :Url:%s,error: %s\n", Url, err)
		if logger != nil {
			logger.Errorf("不能创建Http Request :Url:%s,error: %s\n", Url, err)
		}
		return -1
	}

	var rep *http.Response = nil
	client := &http.Client{}
	for i := retry_max_num; i > 0; i-- {
		rep, err = client.Do(req)
		if err != nil {
			stdoutLogger.Errorf("不能连接到指定的Url :%s,error :%s\n\tRetry it....\n", Url, err)
			if logger != nil {
				logger.Errorf("不能连接到指定的Url :%s,error :%s\n\tRetry it....\n", Url, err)
			}
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
		stdoutLogger.Errorf("不能创建Http Request :Url:%s,error: %s\n", Url, err)
		if logger != nil {
			logger.Errorf("不能创建Http Request :Url:%s,error: %s\n", Url, err)
		}
		return -1
	}
	req.Header.Add("Range", "bytes=0-1")

	var rep *http.Response = nil
	client := &http.Client{}
	for i := retry_max_num; i > 0; i-- {
		rep, err = client.Do(req)
		if err != nil {
			stdoutLogger.Errorf("不能连接到指定的Url :%s,error :%s\n\tRetry it....\n", Url, err)
			if logger != nil {
				logger.Errorf("不能连接到指定的Url :%s,error :%s\n\tRetry it....\n", Url, err)
			}
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
func Ventilator(item ResourceInfo, RoutineCount int, rate chan int64) {

	defer func() {
		if err := recover(); err != nil {
			stdoutLogger.Errorf("A Ventilator panic an error , Seq :%d, error :%s,retry it !!\n", item.Sequence, err)
			if logger != nil {
				logger.Errorf("A Ventilator panic an error , Seq :%d, error :%s,retry it !!", item.Sequence, err)
			}
			panic(err)
			rate <- -1
		}
	}()

	//创建文件
	file_name := fmt.Sprintf("[第%d集]%s.%s", item.Sequence, item.Name, item.Suffix)
	file_name = filtLegalString(file_name)
	file, err := os.OpenFile(file_name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	defer file.Close()
	if err != nil {
		if os.IsExist(err) {
			stdoutLogger.Errorf("%s 已经存在,它将不会被重新下载!!\n", file_name)
			if logger != nil {
				logger.Errorf("%s 已经存在,它将不会被重新下载!!\n", file_name)
			}
			rate <- 0
			return
		} else {
			stdoutLogger.Errorf("os.OpenFile() ,file_name :%s ,error :%s\n", file_name, err)
			if logger != nil {
				logger.Errorf("os.Create() ,file_name :%s ,error :%s\n", file_name, err)
			}
			rate <- -1
			return
		}
	}

	stdoutLogger.Infof("开始下载视频:%s\n", item.Name)
	if logger != nil {
		logger.Infof("开始下载视频:%s\n", item.Name)
	}
	//获得目标文件大小
	ContentLength := GetTargetResourceContentLength(item.DownUrl)
	if ContentLength == 0 {

		stdoutLogger.Infof("目标文件大小未知,只能使用单线程下载\n")
		if logger != nil {
			logger.Infof("目标文件大小未知,只能使用单线程下载\n")
		}

		RoutineCount = 1
	} else if ContentLength > 0 {

		stdoutLogger.Printf("文件名:%s\n\t大小: %d Mb\n\tBytes : %d bytes\n\tUrl: %s\n", file_name, (ContentLength / (1024 * 1024)), ContentLength, item.DownUrl)
		if logger != nil {
			logger.Infof("文件名:%s\n\t大小: %d Mb\n\tBytes : %d bytes\n\tUrl: %s\n", file_name, (ContentLength / (1024 * 1024)), ContentLength, item.DownUrl)
		}

		if RoutineCount > 1 { //即使服务端支持多线程，只有routineCount大于1才改变大小
			err = file.Truncate(ContentLength) //使用大小填充指定文件
			if err != nil {

				stdoutLogger.Errorf("不能改变指定文件大小 :%s\n", err)
				if logger != nil {
					logger.Errorf("不能改变指定文件大小 :%s\n", err)
				}

				rate <- -1
				return
			}
		}
	} else if ContentLength == -1 {
		rate <- -1
		return
	}

	//测试目标是否支持Http Header的Range属性
	IsSupport := IsSupportMultiThread(item.DownUrl)
	if IsSupport == 0 {
		RoutineCount = 1

		stdoutLogger.Infof("目标主机不支持多线程下载!,将使用单线程下载\n")
		if logger != nil {
			logger.Infof("目标主机不支持多线程下载!,将使用单线程下载\n")
		}

	} else if IsSupport == 1 {
		stdoutLogger.Infof("目标主机支持多线程下载\n")
		if logger != nil {
			logger.Infof("目标主机支持多线程下载\n")
		}
	} else if IsSupport == -1 {
		rate <- -1
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
					stdoutLogger.Infof("文件 %s 的下载Routine下载成功，消耗时间:%d s\n", file_name, (endTime-startTime)/int64(time.Second))
					if logger != nil {
						logger.Infof("文件 %s 的下载Routine下载成功，消耗时间:%d s\n", file_name, (endTime-startTime)/int64(time.Second))
					}
					break
				}
			}
			if i < 0 {
				rate <- -1
			} else {
				elapsed_senconds := ((endTime - startTime) / int64(time.Second))
				Stat, _ := file.Stat()
				rate <- Stat.Size() / 1024 / elapsed_senconds
			}
		} else if ContentLength == 0 { //目标文件大小未知

			//创建goroutine下载文件
			var i int = 0
			for i = retry_max_num; i > 0; i-- {
				go Worker(file, item.DownUrl, 0, 0, channel)
				r := <-channel
				if r != -1 { //routine成功下载文件
					endTime = time.Now().UnixNano()

					stdoutLogger.Infof("文件 %s 的下载Routine下载成功，消耗时间:%d s\n", file_name, (endTime-startTime)/int64(time.Second))
					if logger != nil {
						logger.Infof("文件 %s 的下载Routine下载成功，消耗时间:%d s\n", file_name, (endTime-startTime)/int64(time.Second))
					}

					break
				}
			}
			if i < 0 {
				rate <- -1
			} else {
				elapsee_senconds := ((endTime - startTime) / int64(time.Second))
				Stat, _ := file.Stat()
				rate <- Stat.Size() / 1024 / elapsee_senconds
			}
		}
	} else { //多线程下载
		start_time := time.Now().UnixNano()
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

			stdoutLogger.Debugf("文件 %s :开始第 %d 个下载Routine\n", file_name, i)
			if logger != nil {
				logger.Debugf("文件 %s :开始第 %d 个下载Routine\n", file_name, i)
			}

			files[i], err = os.OpenFile(file_name, os.O_WRONLY, 0666)
			defer files[i].Close()
			if err != nil {

				stdoutLogger.Errorf("os.OpenFile() :不能打开指定文件 :%s\n", err)
				if logger != nil {
					logger.Errorf("os.OpenFile() :不能打开指定文件 :%s\n", err)
				}
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

		stdoutLogger.Infof("创建%d个下载routine完成,开始下载!!!\n", RoutineCount)
		if logger != nil {
			logger.Infof("创建%d个下载routine完成,开始下载!!!\n", RoutineCount)
		}

		//等待Routines返回

		for i, v := range channels {

			stdoutLogger.Debugf("文件 %s :开始等待第 %d 个下载Routine退出\n", file_name, i)
			if logger != nil {
				logger.Debugf("文件 %s :开始等待第 %d 个下载Routine退出\n", file_name, i)
			}

			r := <-v
			if r != -1 { //routine成功下载文件
				stdoutLogger.Infof("文件 %s 的 第 %d 个下载Routine下载成功，消耗时间:%d s,平均下载速度:%d Kb/s\n", file_name, i, r, (block/1000)/r)
				if logger != nil {
					logger.Infof("文件 %s 的 第 %d 个下载Routine下载成功，消耗时间:%d s,平均下载速度:%d Kb/s\n", file_name, i, r, (block/1000)/r)
				}

			} else { //该routine失败,重试之
				stdoutLogger.Errorf("文件 %s 的 第 %d 个下载Routine下载失败,重试...\n", file_name, i)
				if logger != nil {
					logger.Errorf("文件 %s 的 第 %d 个下载Routine下载失败,重试...\n", file_name, i)
				}
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
						stdoutLogger.Infof("文件 %s 的 第 %d 个下载Routine重新下载成功，消耗时间:%d s,平均下载速度:%d Kb/s\n", file_name, i, new_r, (block/1024)/new_r)
						if logger != nil {
							logger.Infof("文件 %s 的 第 %d 个下载Routine重新下载成功，消耗时间:%d s,平均下载速度:%d Kb/s\n", file_name, i, new_r, (block/1024)/new_r)
						}
						break
					}
				}

				if j < 0 { //下载分段失败
					errInfo := fmt.Sprintf("文件: %s ,Range :%d-%d :下载失败\n", file_name, start, end)
					logFile, err := os.OpenFile(item.Name+".err", os.O_APPEND|os.O_WRONLY, 0666)
					defer logFile.Close()
					if err != nil {
						fmt.Printf("创建Log文件失败: %s\n", err)
					} else {
						logFile.WriteString(errInfo)
					}
				}

			}
		}
		end_time := time.Now().UnixNano()
		elapsee_senconds := (end_time - start_time) / int64(time.Second)
		Stat, _ := file.Stat()
		rate <- Stat.Size() / 1024 / elapsee_senconds

	}
}

func Worker(file *os.File, Url string, startOffset, endOffset int64, channel chan int64) {
	defer func() {
		if err := recover(); err != nil {
			stdoutLogger.Errorf("A worker panic an error , range :%d-%d,error :%s,retry it !!", startOffset, endOffset, err)
			if logger != nil {
				logger.Errorf("A worker panic an error , range :%d-%d,error :%s,retry it !!", startOffset, endOffset, err)
			}
			channel <- -1
			return
		}
	}()

	Stat, _ := file.Stat()
	size := Stat.Size()
	if size > 0 && endOffset > startOffset { //多线程

		startTime := time.Now().UnixNano()
		req, err := http.NewRequest("GET", Url, nil)
		if err != nil {

			stdoutLogger.Errorf("不能创建Http Request :Url:%s,error: %s\n", Url, err)
			if logger != nil {
				logger.Errorf("不能创建Http Request :Url:%s,error: %s\n", Url, err)
			}

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
				stdoutLogger.Errorf("不能连接到指定的Url :%s,\n\terror :%s\n\tRetry it....\n", Url, err)
				if logger != nil {
					logger.Errorf("不能连接到指定的Url :%s,\n\terror :%s\n\tRetry it....\n", Url, err)
				}
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

				stdoutLogger.Debugf("File :%s ,Range : %d-%d ,needSize :%d ,written :%d\n", Stat.Name(), startOffset, endOffset, needSize, written)

				if logger != nil {
					logger.Debugf("File :%s ,Range : %d-%d ,needSize :%d ,written :%d\n", Stat.Name(), startOffset, endOffset, needSize, written)
				}
				file.Sync()
				break
			} else if err != nil {
				stdoutLogger.Errorf("io.CopyN() error: %s Range : %d - %d ,error:%s ,written :%d,Retry it !!!\n", Stat.Name(), startOffset, endOffset, err, needSize, written)
				if logger != nil {
					logger.Errorf("io.CopyN() error: %s Range : %d - %d ,error:%s,written :%d,Retry it !!!\n", Stat.Name(), startOffset, endOffset, err, needSize, written)
				}
			} /* else if err == io.EOF {

				stdoutLogger.Debugf("File :%s ,Range : %d-%d ,needSize :%d ,written :%d, error: io.EOF\n", Stat.Name(), startOffset, endOffset, needSize, written)

				if logger != nil {
					logger.Debugf("File :%s ,Range : %d-%d ,needSize :%d ,written :%d, error: io.EOF\n", Stat.Name(), startOffset, endOffset, needSize, written)
				}
				break
			}*/
		}

		defer rep.Body.Close()
		if i < 0 {
			channel <- -1
			return
		} else {

			endTime := time.Now().UnixNano()
			stdoutLogger.Debugf("Download the Range  is successful : File '%s',  Range : %s , Elapsed_time : %ds\n", file.Name(), RangeValue, (endTime-startTime)/int64(time.Second))
			if logger != nil {
				logger.Debugf("Download the Range  is successful : File '%s',  Range : %s , Elapsed_time : %ds\n", file.Name(), RangeValue, (endTime-startTime)/int64(time.Second))
			}
			channel <- (endTime - startTime) / int64(time.Second)
			return
		}

	} else { //单线程
		start := time.Now().UnixNano()
		end := time.Now().UnixNano()
		stdoutLogger.Info("开始单线程下载......")
		if logger != nil {
			logger.Info("开始单线程下载......")
		}
		var rep *http.Response = nil
		var err error
		for i := retry_max_num; i > 0; i-- {
			rep, err = http.Get(Url)
			if err != nil {
				stdoutLogger.Errorf("不能连接到指定的Url :%s,\n\terror :%s\n\tRetry it....\n", Url, err)
				if logger != nil {
					logger.Errorf("不能连接到指定的Url :%s,\n\terror :%s\n\tRetry it....\n", Url, err)
				}
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
					file.Sync()
					break
				} else {
					stdoutLogger.Errorf("io.CopyN() error: %s\n", err)
					if logger != nil {
						logger.Errorf("io.CopyN() error: %s\n", err)
					}
					//panic(err)
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
					file.Sync()
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
	fmt.Printf("%s\t--url\t[-rnum]\t[-level --file]\t[--verbose]\n", strings.Join(os.Args[0:1], ""))
	fmt.Printf("\t--url :\n")
	fmt.Printf("\t\tThe url of opencourse\n")
	fmt.Printf("\t--rnum :\n")
	fmt.Printf("\t\tThe number of threads\n\t\tThe param is optional,and the default value is 10\n")
	fmt.Print("\t--verbose :\n")
	fmt.Printf("\t\tThe verbose mode\n")
	fmt.Printf("\t--file :\n")
	fmt.Printf("\t\twhether written the log to file\n\t\tNote : if the flag is setted ,the level flag must be setted too!!! \n")
	fmt.Printf("\t--level :\n")
	fmt.Printf("\t\tThe level of log\n\t\tNote:The flags is available only when the file flag be setted\n\n")
	fmt.Printf("\t\t%d : Debug mode\n", Ldebug)
	fmt.Printf("\t\t%d : Info mode\n", Linfo)
	fmt.Printf("\t\t%d : Error mode\n", Lerror)
	fmt.Printf("\t--episode :\n")
	fmt.Printf("\t\tthe episode what you need to download\n\n")
	fmt.Printf("\t\tIf you only need one episode ,the value should be a number\n")
	fmt.Printf("\t\tIf you need multi-episode,the value should be many numbers, and splitd by ','\n")
	fmt.Printf("\t\tIf you need a range ,the split flag is '-'\n\n")
	fmt.Printf("\t\tfor example :\n")
	fmt.Printf("\t\t--episode 1     : you only need the episode one \n")
	fmt.Printf("\t\t--episode 1,3,5 : you need the episode one,episode three ,and the episode five\n")
	fmt.Printf("\t\t--episode 1-10  : you need one range ,and it is  : 1-10\n")
	fmt.Printf("\t\t--episode 1-    : you need those episodes : from episode one to the last episode\n")
	fmt.Printf("\nFor example : \n\t%s --url http://v.163.com/special/justice/ --rnum 20\n", strings.Join(os.Args[0:1], ""))
}

func NewLogFile(name string, level int) *os.File {
	suffix := level_suffix[level]
	logFile, err := os.OpenFile(name+suffix, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		fmt.Printf("创建Log文件失败: %s\n", err)
		return nil
	} else {
		return logFile
	}
}
func main() {

	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("Main() panic an error , error :%s\n", err)
			os.Exit(1)
		}
	}()

	if len(os.Args) < 2 {
		Usage()
		return
	}

	var Url, EpisodeValue string
	var RoutineNum, Level int
	var IsLogToFile, IsVerbose bool
	flag.StringVar(&Url, "url", "", "the url of opencourse")
	flag.IntVar(&RoutineNum, "rnum", default_routine_num, "the number of threads")
	flag.IntVar(&Level, "level", -1, "the log level that recording to file ")
	flag.BoolVar(&IsLogToFile, "file", false, "whether written the log to file,Note : if the flag is setted ,the level flag must be too!!! ")
	flag.BoolVar(&IsVerbose, "verbose", false, "verbose mode")
	flag.StringVar(&EpisodeValue, "episode", "", "the episode what you need to download")
	flag.Parse()

	//参数检查
	if len(Url) == 0 {
		Usage()
		return
	}
	if IsVerbose {
		is_verbose = true
	}
	if IsLogToFile && Level == -1 {
		Usage()
		return
	}

	var e EpisodeList

	var stdoutLogger_level int = logex.Lerror | logex.Linfo
	if is_verbose {
		stdoutLogger_level |= logex.Ldebug
	}
	stdoutLogger = logex.New(os.Stdout, "", 0)
	stdoutLogger.Level = stdoutLogger_level
	//stdoutLogger.SetOutputLevel(stdoutLogger_level)

	sourceCode := getSourceCode(Url)
	list := getResourceDownloadList(sourceCode)

	if e.IsNeedPartial() {
		stdoutLogger.Debugf("Partial mode\n")
		if e.ParseValue(EpisodeValue, len(*list)) {
			fmt.Printf("the value of episode param is incorectly!!!\n")
			Usage()
			return
		}
	} else {
		stdoutLogger.Debugf("normal mode\n")
	}

	fmt.Printf("episode list :%v\n", e.needlist)
	if IsLogToFile && Level != -1 {
		if Level == Linfo ||
			Level == Ldebug ||
			Level == Lerror {
			switch Level {
			case Linfo:
				logfile_level = logex.Linfo
			case Ldebug:
				logfile_level = logex.Linfo | logex.Ldebug //DEBUG模式下同样输出Info信息
			default:
				logfile_level = Level
			}
		} else {
			fmt.Printf("the level value is illegally,it only should be %d ,%d,%d\n", Lerror, Linfo, Ldebug)
			Usage()
			return
		}
	}

	channels := make([]chan int64, 0)
	if len(*list) == 0 {
		fmt.Printf("the resource list is empty!!\n")
		os.Exit(1)
	}
	for i, v := range *list {

		start_time := time.Now().UnixNano()
		if len(e.needlist) != 0 &&
			!e.IsNeedEpisode(i+1) {
			continue

		}
		if IsLogToFile && Level != -1 {
			logfile := NewLogFile(v.Name, Level)
			defer logfile.Close()
			logger = logex.New(logfile, "", logex.Ldate|logex.Ltime|logex.Lshortfile|logex.Llevel) //保存logger
			logger.Level = logfile_level
		}
		channel := make(chan int64)
		channels = append(channels, channel)
		for j := retry_max_num; j > 0; j-- {
			go Ventilator(v, RoutineNum, channel)
			r := <-channel
			if r != -1 {
				end_time := time.Now().UnixNano()
				elapsed_time := (end_time - start_time) / int64(time.Second)
				msg := fmt.Sprintf("文件 ‘%s’ 下载完成!!! 消耗时间:%ds,平均下载速度:%d Kb/s\n", (*list)[i].Name+"."+(*list)[i].Suffix, elapsed_time, r)
				stdoutLogger.Infof("%s", msg)
				if logger != nil {
					logger.Infof("%s", msg)
				}
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
